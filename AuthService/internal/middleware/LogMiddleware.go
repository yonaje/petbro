package middleware

import (
	"net/http"
	"time"

	"github.com/yonaje/authservice/internal/logger"
	"github.com/yonaje/authservice/internal/metrics"
	"go.uber.org/zap"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

func Logging(log *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)
		metrics.ObserveHTTPRequest(r, rw.status, time.Since(start), rw.bytes)

		reqLog := logger.WithTrace(r.Context(), log)
		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.status),
			zap.Int("bytes", rw.bytes),
			zap.Duration("duration", time.Since(start)),
		}

		switch {
		case rw.status >= 500:
			reqLog.Error("http request completed", fields...)
		case rw.status >= 400:
			reqLog.Warn("http request completed", fields...)
		default:
			reqLog.Info("http request completed", fields...)
		}
	})
}
