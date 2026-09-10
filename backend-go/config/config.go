package config

import (
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
  jwt_secret: "dev-insecure-secret-change-me"
  access_ttl: "1h"
  bcrypt_cost: 12
`)

type Config struct {
	Application string   `koanf:"application"`
	Listen      string   `koanf:"listen"`
	IsProdMode  bool     `koanf:"is_prod_mode"`
	Logger      Logger   `koanf:"logger"`
	Postgres    Postgres `koanf:"postgres"`
	Auth        Auth     `koanf:"auth"`
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
