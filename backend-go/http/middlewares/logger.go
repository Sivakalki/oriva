// Package middlewares holds the service's HTTP middleware.
package middlewares

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	reqDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	reqTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests.",
	}, []string{"method", "route", "status"})
)

// LoggerWithMetrics logs each request and records Prometheus metrics. The route
// label uses the chi route pattern (not the raw path) to bound cardinality.
func LoggerWithMetrics(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				dur := time.Since(start)
				route := chi.RouteContext(r.Context()).RoutePattern()
				if route == "" {
					route = "unmatched"
				}
				status := strconv.Itoa(ww.Status())

				reqDuration.WithLabelValues(r.Method, route, status).Observe(dur.Seconds())
				reqTotal.WithLabelValues(r.Method, route, status).Inc()

				fields := []zap.Field{
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("route", route),
					zap.Int("status", ww.Status()),
					zap.Int("bytes", ww.BytesWritten()),
					zap.Int64("duration_ms", dur.Milliseconds()),
					zap.String("request_id", middleware.GetReqID(r.Context())),
				}
				if isMonitoringPath(r.URL.Path) {
					logger.Debug("served", fields...)
				} else {
					logger.Info("served", fields...)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func isMonitoringPath(p string) bool {
	return strings.HasSuffix(p, "/health") || strings.HasSuffix(p, "/metrics")
}
