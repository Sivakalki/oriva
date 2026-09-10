// Command seed provisions a dev organization and one scheduler user.
//
// Env:
//
//	POSTGRES_DSN            (falls back to the config default)
//	SEED_ORG_NAME           (default "Dev Org")
//	SEED_SCHEDULER_EMAIL    (required)
//	SEED_SCHEDULER_PASSWORD (required)
package main

import (
	"context"
	"log"
	"os"
	"time"

	"oriva/backend-go/config"
	"oriva/backend-go/db/postgres"
	"oriva/backend-go/utils/helpers"
	"oriva/backend-go/utils/jwt"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
)

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		k := koanf.New(".")
		_ = k.Load(rawbytes.Provider(config.DefaultConfig), yaml.Parser())
		var cfg config.Config
		_ = k.Unmarshal("", &cfg)
		dsn = cfg.Postgres.DSN
	}

	orgName := envOr("SEED_ORG_NAME", "Dev Org")
	email := os.Getenv("SEED_SCHEDULER_EMAIL")
	password := os.Getenv("SEED_SCHEDULER_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("SEED_SCHEDULER_EMAIL and SEED_SCHEDULER_PASSWORD are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := postgres.Connect(ctx, config.Postgres{DSN: dsn, MaxConns: 2})
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	orgID, err := postgres.NewOrgRepo(pool).UpsertByName(ctx, orgName)
	if err != nil {
		log.Fatalf("upsert org: %v", err)
	}

	hash, err := helpers.HashPassword(password, 12)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	u, err := postgres.NewUserRepo(pool).Upsert(ctx, orgID, email, hash, jwt.RoleScheduler)
	if err != nil {
		log.Fatalf("upsert user: %v", err)
	}

	log.Printf("seeded org %q (%s) and scheduler %s (%s)", orgName, orgID, u.Email, u.ID)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
