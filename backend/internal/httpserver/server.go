package httpserver

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

type Config struct {
	AllowedOrigin string
}

type Server struct {
	processor     *disbursement.Processor
	logger        *slog.Logger
	allowedOrigin string
	mux           *http.ServeMux
}

var ErrInvalidServer = errors.New("invalid HTTP server")

func New(processor *disbursement.Processor, logger *slog.Logger, config Config) (*Server, error) {
	if processor == nil {
		return nil, fmt.Errorf("%w: processor is required", ErrInvalidServer)
	}
	if logger == nil {
		return nil, fmt.Errorf("%w: logger is required", ErrInvalidServer)
	}
	if strings.TrimSpace(config.AllowedOrigin) == "" {
		return nil, fmt.Errorf("%w: allowed origin is required", ErrInvalidServer)
	}

	server := &Server{
		processor:     processor,
		logger:        logger,
		allowedOrigin: config.AllowedOrigin,
		mux:           http.NewServeMux(),
	}
	server.mux.HandleFunc("GET /workers", server.listWorkers)

	return server, nil
}

func (s *Server) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	requestID := "req-" + strings.ToLower(rand.Text())
	responseWriter.Header().Set("X-Request-ID", requestID)

	if request.Header.Get("Origin") == s.allowedOrigin {
		responseWriter.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
		responseWriter.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		responseWriter.Header().Add("Vary", "Origin")
		if request.Method == http.MethodOptions {
			responseWriter.WriteHeader(http.StatusNoContent)
			return
		}
	}

	s.mux.ServeHTTP(responseWriter, request)
}

func (s *Server) listWorkers(responseWriter http.ResponseWriter, _ *http.Request) {
	workers := s.processor.AvailableWorkers()
	response := make([]openapi.Worker, 0, len(workers))
	for _, worker := range workers {
		response = append(response, openapi.Worker{
			Id:       string(worker.ID()),
			Name:     worker.Name(),
			Amount:   worker.Amount().String(),
			Currency: openapi.Currency(worker.Amount().Currency()),
		})
	}

	writeJSON(responseWriter, http.StatusOK, response)
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, body any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	if err := json.NewEncoder(responseWriter).Encode(body); err != nil {
		slog.Error("encode JSON response", "error", err)
	}
}
