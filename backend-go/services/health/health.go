// Package health reports whether the service's dependencies are reachable.
package health

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// pinger is the dependency contract, defined at the point of consumption so it
// is trivially fakeable in tests. *pgxpool.Pool satisfies it directly.
type pinger interface {
	Ping(ctx context.Context) error
}

// Service checks liveness of the datastore.
type Service struct {
	db     pinger
	logger *zap.Logger
}

// NewService constructs a health Service.
func NewService(logger *zap.Logger, db pinger) *Service {
	return &Service{db: db, logger: logger}
}

// Health reports whether the datastore responds within a short timeout.
func (s *Service) Health(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		if s.logger != nil {
			s.logger.Warn("health check failed", zap.Error(err))
		}
		return false
	}
	return true
}
