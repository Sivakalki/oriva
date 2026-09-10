package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func base() Config {
	return Config{
		Application: "oriva-backend",
		Listen:      ":8080",
		Logger:      Logger{Level: "debug"},
		Postgres:    Postgres{DSN: "postgres://u:p@localhost:5432/db?sslmode=disable", MaxConnLifetime: "1h"},
		Auth:        Auth{JWTSecret: "secret", AccessTTL: "1h", BcryptCost: 12},
	}
}

func TestValidate_Happy(t *testing.T) {
	c := base()
	require.NoError(t, c.Validate())
	assert.Equal(t, time.Hour, c.Auth.AccessTTLDur)
	assert.Equal(t, time.Hour, c.Postgres.MaxConnLifetimeDur)
	assert.NotEmpty(t, c.Logger.HostName)
}

func TestValidate_Errors(t *testing.T) {
	cases := map[string]func(*Config){
		"empty listen":        func(c *Config) { c.Listen = "" },
		"bad logger level":    func(c *Config) { c.Logger.Level = "bogus" },
		"empty dsn":           func(c *Config) { c.Postgres.DSN = "" },
		"unparseable dsn":     func(c *Config) { c.Postgres.DSN = "://nope" },
		"bad access_ttl":      func(c *Config) { c.Auth.AccessTTL = "10 flurbs" },
		"empty jwt secret":    func(c *Config) { c.Auth.JWTSecret = "" },
		"bcrypt cost too low": func(c *Config) { c.Auth.BcryptCost = 4; _ = c },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			c := base()
			mut(&c)
			assert.Error(t, c.Validate())
		})
	}
}

func TestValidate_ProdSecretLength(t *testing.T) {
	c := base()
	c.IsProdMode = true
	c.Auth.JWTSecret = "short"
	assert.Error(t, c.Validate())

	c.Auth.JWTSecret = "0123456789012345678901234567890123"
	assert.NoError(t, c.Validate())
}

func TestValidate_BcryptCostDefault(t *testing.T) {
	c := base()
	c.Auth.BcryptCost = 0
	require.NoError(t, c.Validate())
	assert.Equal(t, 12, c.Auth.BcryptCost)
}
