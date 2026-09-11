package scoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var scoreBuckets = []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

var (
	turnScoreHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "oriva_turn_score",
		Help:    "Per-turn interview answer score (0-100) from the judge LLM.",
		Buckets: scoreBuckets,
	})
	overallScoreHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "oriva_overall_score",
		Help:    "Overall interview score (0-100) from the judge LLM.",
		Buckets: scoreBuckets,
	})
	scoringDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "oriva_scoring_duration_seconds",
		Help:    "Judge LLM call latency, by stage.",
		Buckets: prometheus.DefBuckets,
	}, []string{"stage"}) // stage: turn | overall

	scoringErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oriva_scoring_errors_total",
		Help: "Scoring failures, by stage.",
	}, []string{"stage"}) // stage: turn | overall
)
