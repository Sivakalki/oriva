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
	"oriva/backend-go/services/auth"
	"oriva/backend-go/services/candidates"
	"oriva/backend-go/services/health"
	"oriva/backend-go/services/interviews"
	"oriva/backend-go/services/jobs"
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

	jwtSvc := jwt.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTTLDur)

	userRepo := postgres.NewUserRepo(pool)
	jobRepo := postgres.NewJobRepo(pool)
	candRepo := postgres.NewCandidateRepo(pool)
	interviewRepo := postgres.NewInterviewRepo(pool)

	hs := apxhttp.Handlers{
		Auth:       handlers.NewAuthHandler(auth.NewService(userRepo, jwtSvc)),
		Candidates: handlers.NewCandidatesHandler(candidates.NewService(candRepo)),
		Health:     handlers.NewHealthHandler(health.NewService(logger, pool)),
		Interviews: handlers.NewInterviewsHandler(interviews.NewService(interviewRepo, jobRepo, candRepo)),
		Jobs:       handlers.NewJobsHandler(jobs.NewService(jobRepo)),
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
