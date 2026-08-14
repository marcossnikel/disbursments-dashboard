// Package httpserver adapts the disbursement application to its HTTP API.
package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

// Config contains HTTP adapter configuration.
type Config struct {
	AllowedOrigin string
}

// Server routes and translates HTTP requests for a disbursement processor.
type Server struct {
	processor     *disbursement.Processor
	logger        *slog.Logger
	allowedOrigin string
	mux           *http.ServeMux
}

// ErrInvalidServer reports missing server dependencies or configuration.
var ErrInvalidServer = errors.New("invalid HTTP server")

// New validates dependencies and creates the HTTP handler.
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
	server.registerRoutes()

	return server, nil
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /workers", s.handleListWorkers)
	s.mux.HandleFunc("POST /disbursements", s.handleSubmitDisbursements)
	s.mux.HandleFunc("GET /disbursements/{batch_id}", s.handleGetDisbursementBatch)
	s.mux.HandleFunc("POST /demo/reset", s.handleResetDemo)
}
