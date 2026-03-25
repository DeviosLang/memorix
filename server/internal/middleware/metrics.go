package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/service"
)

// MetricsRecorder is an interface for recording metrics.
// This allows the middleware to work with any metrics implementation.
type MetricsRecorder interface {
	RecordRequest(metric domain.RequestMetric)
	RecordSearch(metric domain.SearchMetric)
	RecordConflict(metric domain.ConflictMetric)
}

// MetricsMiddleware creates a middleware that collects request-level metrics.
func MetricsMiddleware(recorder MetricsRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			// Call the next handler
			next.ServeHTTP(ww, r)

			// Record the request metric
			duration := time.Since(start)

			// Get auth info if available
			var tenantID, agentID string
			if auth := AuthFromContext(r.Context()); auth != nil {
				tenantID = auth.TenantID
				agentID = auth.AgentName
			}

			recorder.RecordRequest(domain.RequestMetric{
				Path:       r.URL.Path,
				Method:     r.Method,
				StatusCode: ww.Status(),
				LatencyMs:  float64(duration.Milliseconds()),
				Timestamp:  start,
				TenantID:   tenantID,
				AgentID:    agentID,
			})
		})
	}
}

// SearchMetricsRecorder provides methods for recording search-specific metrics.
// This is used by the memory service to record search operations.
type SearchMetricsRecorder struct {
	collector *service.MetricsCollector
}

// NewSearchMetricsRecorder creates a new search metrics recorder.
func NewSearchMetricsRecorder(collector *service.MetricsCollector) *SearchMetricsRecorder {
	return &SearchMetricsRecorder{collector: collector}
}

// RecordSearch records a search operation metric.
func (r *SearchMetricsRecorder) RecordSearch(searchType string, latencyMs float64, success bool, tenantID string) {
	r.collector.RecordSearch(domain.SearchMetric{
		SearchType: searchType,
		LatencyMs:  latencyMs,
		Success:    success,
		Timestamp:  time.Now(),
		TenantID:   tenantID,
	})
}

// ConflictMetricsRecorder provides methods for recording conflict resolution metrics.
type ConflictMetricsRecorder struct {
	collector *service.MetricsCollector
}

// NewConflictMetricsRecorder creates a new conflict metrics recorder.
func NewConflictMetricsRecorder(collector *service.MetricsCollector) *ConflictMetricsRecorder {
	return &ConflictMetricsRecorder{collector: collector}
}

// RecordConflict records a conflict resolution metric.
func (r *ConflictMetricsRecorder) RecordConflict(factID, userID, resolution string, success bool, tenantID string) {
	r.collector.RecordConflict(domain.ConflictMetric{
		FactID:     factID,
		UserID:     userID,
		Resolution: resolution,
		Success:    success,
		Timestamp:  time.Now(),
		TenantID:   tenantID,
	})
}

// MetricsResponseWriter wraps a response writer to capture additional metrics.
type MetricsResponseWriter struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

// WriteHeader captures the status code.
func (w *MetricsResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the bytes written.
func (w *MetricsResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

// Status returns the captured status code.
func (w *MetricsResponseWriter) Status() int {
	if !w.wroteHeader {
		return http.StatusOK
	}
	return w.status
}

// Written returns the total bytes written.
func (w *MetricsResponseWriter) Written() int64 {
	return w.written
}

// Hijack implements the http.Hijacker interface.
func (w *MetricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("response writer does not implement http.Hijacker")
}

// Flush implements the http.Flusher interface.
func (w *MetricsResponseWriter) Flush() {
	if fw, ok := w.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}
