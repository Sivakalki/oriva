// Package postgres owns the pgx connection pool and the sentinel errors every
// entity repository under repositories/postgres/* translates its pgx errors
// into. Each entity (user, org, job, candidate, interview, response, state)
// lives in its own subpackage; this parent package is their shared, DB-only
// infrastructure so none of them need to depend on each other.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"oriva/backend-go/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo-level sentinels. Services translate these into transport-agnostic
// *apxerrors.Error values.
var (
	// ErrNotFound is returned when a lookup matches no row.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned on a unique-constraint violation.
	ErrConflict = errors.New("conflict")
)

// PgUniqueViolation is the SQLSTATE for a unique_violation.
const PgUniqueViolation = "23505"

// Connect builds a pool from cfg and verifies it with a Ping.
func Connect(ctx context.Context, cfg config.Postgres) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetimeDur > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetimeDur
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
