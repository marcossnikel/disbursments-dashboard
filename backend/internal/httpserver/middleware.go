package httpserver

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type requestIDContextKey struct{}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

// ServeHTTP adds request metadata, CORS, and access logging before routing.
func (s *Server) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	requestStartedAt := time.Now()
	requestID := "req-" + strings.ToLower(rand.Text())
	request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, requestID))
	recordedResponse := &responseRecorder{ResponseWriter: responseWriter}
	defer s.logRequest(request, recordedResponse, requestStartedAt)

	recordedResponse.Header().Set("X-Request-ID", requestID)
	s.applyCORS(recordedResponse, request)
	if request.Method == http.MethodOptions && request.Header.Get("Origin") == s.allowedOrigin {
		recordedResponse.WriteHeader(http.StatusNoContent)
		return
	}

	s.mux.ServeHTTP(recordedResponse, request)
}

func (s *Server) applyCORS(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Header.Get("Origin") != s.allowedOrigin {
		return
	}

	responseWriter.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
	responseWriter.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	responseWriter.Header().Add("Vary", "Origin")
}

func (s *Server) logRequest(
	request *http.Request,
	response *responseRecorder,
	startedAt time.Time,
) {
	statusCode := response.statusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	level := slog.LevelInfo
	if statusCode >= http.StatusInternalServerError {
		level = slog.LevelError
	} else if statusCode >= http.StatusBadRequest {
		level = slog.LevelWarn
	}

	s.logger.Log(
		request.Context(),
		level,
		"HTTP request completed",
		"request_id", requestIDFrom(request),
		"method", request.Method,
		"path", request.URL.Path,
		"status", statusCode,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if r.statusCode != 0 {
		return
	}
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.statusCode == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

func requestIDFrom(request *http.Request) string {
	requestID, _ := request.Context().Value(requestIDContextKey{}).(string)
	return requestID
}
