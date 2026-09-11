package config

import (
	"net/url"
	"os"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/utils/helpers"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultConfig is the baseline, overridden by config/<APX_CONFIG_FILE>.
var DefaultConfig = []byte(`
application: "oriva-backend"
listen: ":8080"
is_prod_mode: false

logger:
  level: "debug"

postgres:
  dsn: "postgres://oriva:oriva@localhost:5432/oriva?sslmode=disable"
  max_conns: 10
  min_conns: 2
  max_conn_lifetime: "1h"
  auto_migrate: false

auth:
  jwt_secret: "asdfnaasdfaeradfaerg"
  access_ttl: "1h"
  bcrypt_cost: 12

mcp:
  enabled: true
  path: "/mcp"
  auth_token: "dev-mcp-token"

app:
  base_url: "http://localhost:5173"
  ai_ws_url: "ws://localhost:8090/ws"

notify:
  transport: "log"
  from: "interviews@oriva.dev"

judge:
  openrouter:
    base_url: "https://openrouter.ai/api/v1"
    api_key: ""
    model: "openai/gpt-4o-mini"
  ollama:
    base_url: "http://localhost:11434/v1"
    api_key: "ollama"
    model: "llama3.2:3b"
`)

type Config struct {
	Application string   `koanf:"application"`
	Listen      string   `koanf:"listen"`
	IsProdMode  bool     `koanf:"is_prod_mode"`
	Logger      Logger   `koanf:"logger"`
	WebApp      WebApp   `koanf:"app"`
	Postgres    Postgres `koanf:"postgres"`
	Auth        Auth     `koanf:"auth"`
	MCP         MCP      `koanf:"mcp"`
	Notify      Notify   `koanf:"notify"`
	Judge       Judge    `koanf:"judge"`
}

// Judge configures the LLM used to score candidate answers (services/scoring).
// OpenRouter is the primary judge; Ollama is the local fallback used when
// openrouter.api_key is empty (or if an OpenRouter call fails at runtime).
// Both speak the OpenAI-compatible chat-completions API. Set the API key via
// env: ORIVA__JUDGE__OPENROUTER__API_KEY (see .env.example), never in YAML.
type Judge struct {
	OpenRouter JudgeProvider `koanf:"openrouter"`
	Ollama     JudgeProvider `koanf:"ollama"`
}

type JudgeProvider struct {
	BaseURL string `koanf:"base_url"`
	APIKey  string `koanf:"api_key"`
	Model   string `koanf:"model"`
}

// WebApp holds settings about the frontend the backend needs (e.g. to build
// candidate-facing URLs).
type WebApp struct {
	BaseURL string `koanf:"base_url"`
	AIWsURL string `koanf:"ai_ws_url"` // ai-service websocket endpoint for the call UI
}

type MCP struct {
	Enabled   bool   `koanf:"enabled"`
	Path      string `koanf:"path"`
	AuthToken string `koanf:"auth_token"`
}

// Notify configures outbound email (candidate invites).
type Notify struct {
	Transport string `koanf:"transport"` // "log" | "smtp"
	From      string `koanf:"from"`
	SMTP      SMTP   `koanf:"smtp"`
}

type SMTP struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Username string `koanf:"username"`
	Password string `koanf:"password"`
}

type Logger struct {
	Level    string `koanf:"level"`
	HostName string `koanf:"host_name"`
}

type Postgres struct {
	DSN             string `koanf:"dsn"`
	MaxConns        int32  `koanf:"max_conns"`
	MinConns        int32  `koanf:"min_conns"`
	MaxConnLifetime string `koanf:"max_conn_lifetime"`
	AutoMigrate     bool   `koanf:"auto_migrate"`

	MaxConnLifetimeDur time.Duration `koanf:"-"`
}

type Auth struct {
	JWTSecret  string `koanf:"jwt_secret"`
	AccessTTL  string `koanf:"access_ttl"`
	BcryptCost int    `koanf:"bcrypt_cost"`

	AccessTTLDur time.Duration `koanf:"-"`
}

// Validate checks the configuration and populates derived fields.
func (c *Config) Validate() error {
	ve := apxerrors.ValidationErrs()

	if c.Application == "" {
		c.Application = "oriva-backend"
	}
	if c.Listen == "" {
		ve.Add("listen", "cannot be empty")
	}
	if c.Logger.Level == "" {
		ve.Add("logger.level", "cannot be empty")
	} else if !helpers.IsValidLoggerLevel(c.Logger.Level) {
		ve.Add("logger.level", "invalid level")
	}

	if c.Postgres.DSN == "" {
		ve.Add("postgres.dsn", "cannot be empty")
	} else if _, err := pgxpool.ParseConfig(c.Postgres.DSN); err != nil {
		ve.Add("postgres.dsn", "unparseable: "+err.Error())
	}
	if d, err := time.ParseDuration(orDefault(c.Postgres.MaxConnLifetime, "1h")); err != nil {
		ve.Add("postgres.max_conn_lifetime", "invalid duration")
	} else {
		c.Postgres.MaxConnLifetimeDur = d
	}

	if c.Auth.JWTSecret == "" {
		ve.Add("auth.jwt_secret", "cannot be empty")
	} else if c.IsProdMode && len(c.Auth.JWTSecret) < 32 {
		ve.Add("auth.jwt_secret", "must be at least 32 chars in prod mode")
	}
	if d, err := time.ParseDuration(orDefault(c.Auth.AccessTTL, "1h")); err != nil {
		ve.Add("auth.access_ttl", "invalid duration")
	} else {
		c.Auth.AccessTTLDur = d
	}
	if c.Auth.BcryptCost == 0 {
		c.Auth.BcryptCost = 12
	}
	if c.Auth.BcryptCost < 10 || c.Auth.BcryptCost > 14 {
		ve.Add("auth.bcrypt_cost", "must be between 10 and 14")
	}

	if c.MCP.Enabled {
		if c.MCP.Path == "" {
			c.MCP.Path = "/mcp"
		}
		if c.MCP.Path[0] != '/' {
			ve.Add("mcp.path", "must start with /")
		}
		if c.MCP.AuthToken == "" {
			ve.Add("mcp.auth_token", "cannot be empty when mcp.enabled")
		} else if c.IsProdMode && len(c.MCP.AuthToken) < 16 {
			ve.Add("mcp.auth_token", "must be at least 16 chars in prod mode")
		}
	}

	if c.WebApp.BaseURL == "" {
		c.WebApp.BaseURL = "http://localhost:5173"
	} else if _, err := url.Parse(c.WebApp.BaseURL); err != nil {
		ve.Add("app.base_url", "must be a valid URL")
	}
	if c.WebApp.AIWsURL == "" {
		c.WebApp.AIWsURL = "ws://localhost:8090/ws"
	} else if u, err := url.Parse(c.WebApp.AIWsURL); err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		ve.Add("app.ai_ws_url", "must be a ws:// or wss:// URL")
	}

	if c.Notify.Transport == "" {
		c.Notify.Transport = "log"
	}
	switch c.Notify.Transport {
	case "log":
	case "smtp":
		if c.Notify.SMTP.Host == "" {
			ve.Add("notify.smtp.host", "required when notify.transport is smtp")
		}
		if c.Notify.From == "" {
			ve.Add("notify.from", "required when notify.transport is smtp")
		}
	default:
		ve.Add("notify.transport", "must be log or smtp")
	}
	if c.Notify.From == "" {
		c.Notify.From = "interviews@oriva.dev"
	}

	if c.Judge.Ollama.BaseURL == "" {
		c.Judge.Ollama.BaseURL = "http://localhost:11434/v1"
	}
	if c.Judge.Ollama.APIKey == "" {
		c.Judge.Ollama.APIKey = "ollama"
	}
	if c.Judge.Ollama.Model == "" {
		c.Judge.Ollama.Model = "llama3.2:3b"
	}
	if c.Judge.OpenRouter.BaseURL == "" {
		c.Judge.OpenRouter.BaseURL = "https://openrouter.ai/api/v1"
	}
	if c.Judge.OpenRouter.Model == "" {
		c.Judge.OpenRouter.Model = "openai/gpt-4o-mini"
	}
	// OpenRouter.APIKey is intentionally allowed to be empty here: that's the
	// signal services/llmjudge uses to skip straight to the Ollama fallback,
	// not a validation error.

	if host, err := os.Hostname(); err == nil {
		c.Logger.HostName = host
	} else {
		ve.Add("hostname", "could not be resolved")
	}

	return ve.Err()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
