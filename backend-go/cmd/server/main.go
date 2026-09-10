package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"oriva/backend-go/config"
	"oriva/backend-go/db/postgres"
	apxhttp "oriva/backend-go/http"
	"oriva/backend-go/http/handlers"
	orivamcp "oriva/backend-go/mcp"
	"oriva/backend-go/notify"
	"oriva/backend-go/services/auth"
	"oriva/backend-go/services/candidates"
	"oriva/backend-go/services/health"
	"oriva/backend-go/services/interviews"
	"oriva/backend-go/services/jobs"
	"oriva/backend-go/services/join"
	"oriva/backend-go/services/sessions"
	"oriva/backend-go/statemachine"
	"oriva/backend-go/utils/buildinfo"
	"oriva/backend-go/utils/jwt"

	_ "github.com/jsternberg/zap-logfmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"go.uber.org/zap"
)

func loadConfig() (config.Config, error) {
	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(config.DefaultConfig), yaml.Parser()); err != nil {
		return config.Config{}, err
	}

	name := os.Getenv("APX_CONFIG_FILE")
	if name == "" {
		name = "dev.yaml"
	}
	if path := "config/" + name; fileExists(path) {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return config.Config{}, err
		}
	}

	var cfg config.Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, cfg.Validate()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func newLogger(cfg config.Config) *zap.Logger {
	zc := zap.NewProductionConfig()
	zc.Encoding = "logfmt"
	zc.OutputPaths = []string{"stdout"}
	_ = zc.Level.UnmarshalText([]byte(cfg.Logger.Level))
	zc.InitialFields = map[string]any{
		"service": cfg.Application,
		"host":    cfg.Logger.HostName,
		"version": buildinfo.Version,
	}
	l, err := zc.Build()
	if err != nil {
		log.Fatalf("build logger: %v", err)
	}
	return l
}

func initServer(ctx context.Context, cfg config.Config, logger *zap.Logger) (*apxhttp.Server, error) {
	pool, err := postgres.Connect(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}

	if cfg.Postgres.AutoMigrate {
		logger.Info("running migrations")
		if err := postgres.RunUp(cfg.Postgres.DSN); err != nil {
			return nil, err
		}
	}

	states, transitions, err := postgres.LoadGraph(ctx, pool)
	if err != nil {
		return nil, err
	}
	machine, err := statemachine.New(states, transitions)
	if err != nil {
		return nil, err
	}
	logger.Info("state machine loaded", zap.Int("states", len(states)), zap.Int("transitions", len(transitions)))

	jwtSvc := jwt.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTTLDur)

	userRepo := postgres.NewUserRepo(pool)
	jobRepo := postgres.NewJobRepo(pool)
	candRepo := postgres.NewCandidateRepo(pool)
	interviewRepo := postgres.NewInterviewRepo(pool)

	sessionsSvc := sessions.NewService(interviewRepo, machine)

	notifier, err := notify.NewSender(cfg.Notify, logger)
	if err != nil {
		return nil, err
	}
	interviewsSvc := interviews.NewService(
		interviewRepo, jobRepo, candRepo, notifier, cfg.WebApp.BaseURL, logger)
	joinSvc := join.NewService(interviewRepo)

	hs := apxhttp.Handlers{
		Auth:       handlers.NewAuthHandler(auth.NewService(userRepo, jwtSvc)),
		Candidates: handlers.NewCandidatesHandler(candidates.NewService(candRepo)),
		Health:     handlers.NewHealthHandler(health.NewService(logger, pool)),
		Interviews: handlers.NewInterviewsHandler(interviewsSvc, sessionsSvc),
		Join:       handlers.NewJoinHandler(joinSvc),
		Jobs:       handlers.NewJobsHandler(jobs.NewService(jobRepo)),
		Sessions:   handlers.NewSessionsHandler(sessionsSvc),
	}

	if cfg.MCP.Enabled {
		mcpSrv := orivamcp.NewServer(orivamcp.Deps{
			Interviews: interviewRepo,
			Responses:  postgres.NewResponseRepo(pool),
			Sessions:   sessionsSvc,
			Logger:     logger,
		})
		hs.MCP = orivamcp.BearerAuth(cfg.MCP.AuthToken, mcpSrv.Handler())
		hs.MCPPath = cfg.MCP.Path
		logger.Info("mcp server enabled", zap.String("path", cfg.MCP.Path))
	}

	return apxhttp.NewServer(logger, hs, jwtSvc), nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger := newLogger(cfg)
	defer func() { _ = logger.Sync() }()
	logger.Info("starting",
		zap.String("version", buildinfo.Version),
		zap.String("commit", buildinfo.Commit),
		zap.String("built", buildinfo.Date),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv, err := initServer(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("init server", zap.Error(err))
	}
	if err := srv.Listen(ctx, cfg.Listen); err != nil {
		logger.Fatal("server exited", zap.Error(err))
	}
}
