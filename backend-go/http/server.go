// Package http wires the router, middleware and routes.
package http

import (
	"context"
	"net/http"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/http/handlers"
	"oriva/backend-go/http/middlewares"
	apxresp "oriva/backend-go/http/response"
	"oriva/backend-go/utils/jwt"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Server holds the handler dependencies. Fields in alphabetical order.
type Server struct {
	auth       *handlers.Auth
	candidates *handlers.Candidates
	health     *handlers.Health
	interviews *handlers.Interviews
	jobs       *handlers.Jobs
	jwt        *jwt.JWT
	logger     *zap.Logger
}

// Handlers bundles the HTTP handlers passed to NewServer.
type Handlers struct {
	Auth       *handlers.Auth
	Candidates *handlers.Candidates
	Health     *handlers.Health
	Interviews *handlers.Interviews
	Jobs       *handlers.Jobs
}

// NewServer constructs the Server.
func NewServer(logger *zap.Logger, h Handlers, j *jwt.JWT) *Server {
	return &Server{
		auth:       h.Auth,
		candidates: h.Candidates,
		health:     h.Health,
		interviews: h.Interviews,
		jobs:       h.Jobs,
		jwt:        j,
		logger:     logger,
	}
}

func (s *Server) router() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middlewares.CORS())
	r.Use(middlewares.LoggerWithMetrics(s.logger))
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.health.Check)
		r.Handle("/metrics", promhttp.Handler())

		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", s.toHandlerFunc(s.auth.Login))
			r.With(middlewares.RequireAuth(s.jwt)).Get("/me", s.toHandlerFunc(s.auth.Me))
		})

		r.Group(func(r chi.Router) {
			r.Use(middlewares.RequireAuth(s.jwt), middlewares.RequireRole(jwt.RoleScheduler))

			r.Route("/jobs", func(r chi.Router) {
				r.Post("/", s.toHandlerFunc(s.jobs.Create))
				r.Get("/", s.toHandlerFunc(s.jobs.List))
				r.Get("/{id}", s.toHandlerFunc(s.jobs.Get))
				r.Patch("/{id}", s.toHandlerFunc(s.jobs.Update))
			})

			r.Route("/candidates", func(r chi.Router) {
				r.Post("/", s.toHandlerFunc(s.candidates.Create))
				r.Get("/", s.toHandlerFunc(s.candidates.List))
				r.Get("/{id}", s.toHandlerFunc(s.candidates.Get))
				r.Patch("/{id}", s.toHandlerFunc(s.candidates.Update))
			})

			r.Route("/interviews", func(r chi.Router) {
				r.Post("/", s.toHandlerFunc(s.interviews.Schedule))
				r.Get("/", s.toHandlerFunc(s.interviews.List))
				r.Get("/{id}", s.toHandlerFunc(s.interviews.Get))
			})
		})
	})

	return r
}

// Listen starts the server and shuts it down gracefully when ctx is cancelled.
func (s *Server) Listen(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.router()}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", zap.String("addr", addr))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutting down http server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// toHandlerFunc adapts an (any, int, error) handler to http.HandlerFunc,
// mapping *apxerrors.Error to a structured response and anything else to a 500.
func (s *Server) toHandlerFunc(h func(http.ResponseWriter, *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, status, err := h(w, r)
		if err != nil {
			var appErr *apxerrors.Error
			if apxerrors.As(err, &appErr) {
				apxresp.RespondError(w, appErr)
				return
			}
			s.logger.Error("unhandled handler error", zap.Error(err))
			apxresp.RespondMessage(w, http.StatusInternalServerError, "internal error")
			return
		}
		if body != nil {
			apxresp.RespondJSON(w, status, body)
		}
	}
}
