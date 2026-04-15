package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "petbro",
			Subsystem: "authservice",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests handled by the auth service.",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "petbro",
			Subsystem: "authservice",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency for the auth service.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
	httpResponseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "petbro",
			Subsystem: "authservice",
			Name:      "http_response_size_bytes",
			Help:      "Response size in bytes for the auth service.",
			Buckets:   prometheus.ExponentialBuckets(128, 2, 8),
		},
		[]string{"method", "route", "status"},
	)
	authEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "petbro",
			Subsystem: "authservice",
			Name:      "auth_events_total",
			Help:      "Business events produced by the auth service.",
		},
		[]string{"action", "result"},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		httpResponseSizeBytes,
		authEventsTotal,
	)
}

func ObserveHTTPRequest(r *http.Request, statusCode int, duration time.Duration, responseBytes int) {
	route := r.Pattern
	if route == "" {
		route = r.URL.Path
	}

	status := strconv.Itoa(statusCode)
	labels := []string{r.Method, route, status}

	httpRequestsTotal.WithLabelValues(labels...).Inc()
	httpRequestDuration.WithLabelValues(labels...).Observe(duration.Seconds())
	httpResponseSizeBytes.WithLabelValues(labels...).Observe(float64(responseBytes))
}

func IncAuthEvent(action, result string) {
	authEventsTotal.WithLabelValues(action, result).Inc()
}
