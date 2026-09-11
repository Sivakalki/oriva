package config_test

import (
	"testing"
	"time"

	"oriva/backend-go/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func base() config.Config {
	return config.Config{
		Application: "oriva-backend",
		Listen:      ":8080",
		Logger:      config.Logger{Level: "debug"},
		Postgres:    config.Postgres{DSN: "postgres://u:p@localhost:5432/db?sslmode=disable", MaxConnLifetime: "1h"},
		Auth:        config.Auth{JWTSecret: "secret", AccessTTL: "1h", BcryptCost: 12},
	}
}

func TestValidate_Happy(t *testing.T) {
	c := base()
	require.NoError(t, c.Validate())
	assert.Equal(t, time.Hour, c.Auth.AccessTTLDur)
	assert.Equal(t, time.Hour, c.Postgres.MaxConnLifetimeDur)
	assert.NotEmpty(t, c.Logger.HostName)
	assert.Equal(t, "http://localhost:5173", c.WebApp.BaseURL)
	assert.Equal(t, "ws://localhost:8090/ws", c.WebApp.AIWsURL)
}

func TestValidate_MCP(t *testing.T) {
	c := base()
	c.MCP = config.MCP{Enabled: true, AuthToken: "dev-mcp-token"}
	require.NoError(t, c.Validate())
	assert.Equal(t, "/mcp", c.MCP.Path) // defaulted

	c = base()
	c.MCP = config.MCP{Enabled: false} // disabled + empty token is fine
	assert.NoError(t, c.Validate())

	c = base()
	c.IsProdMode = true
	c.Auth.JWTSecret = "0123456789012345678901234567890123"
	c.MCP = config.MCP{Enabled: true, AuthToken: "short"}
	assert.Error(t, c.Validate())
}

func TestValidate_Errors(t *testing.T) {
	cases := map[string]func(*config.Config){
		"empty listen":        func(c *config.Config) { c.Listen = "" },
		"bad logger level":    func(c *config.Config) { c.Logger.Level = "bogus" },
		"empty dsn":           func(c *config.Config) { c.Postgres.DSN = "" },
		"unparseable dsn":     func(c *config.Config) { c.Postgres.DSN = "://nope" },
		"bad access_ttl":      func(c *config.Config) { c.Auth.AccessTTL = "10 flurbs" },
		"empty jwt secret":    func(c *config.Config) { c.Auth.JWTSecret = "" },
		"bcrypt cost too low": func(c *config.Config) { c.Auth.BcryptCost = 4; _ = c },
		"mcp enabled no token": func(c *config.Config) {
			c.MCP = config.MCP{Enabled: true, Path: "/mcp"}
		},
		"mcp bad path": func(c *config.Config) {
			c.MCP = config.MCP{Enabled: true, Path: "mcp", AuthToken: "tok"}
		},
		"ai_ws_url http scheme": func(c *config.Config) { c.WebApp.AIWsURL = "http://localhost:8090/ws" },
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
