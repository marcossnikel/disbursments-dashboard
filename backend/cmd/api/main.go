package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/demodata"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/httpserver"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/mockpayment"
)

const (
	apiAddressEnvironmentVariable     = "API_ADDRESS"
	frontendOriginEnvironmentVariable = "FRONTEND_ORIGIN"

	defaultAPIAddress     = ":8080"
	defaultFrontendOrigin = "http://localhost:5173"

	providerPaymentTimeout      = time.Second
	providerMaxConcurrentCalls  = 3
	serverShutdownTimeout       = 5 * time.Second
	serverReadHeaderTimeout     = 5 * time.Second
	serverRequestReadTimeout    = 10 * time.Second
	serverResponseWriteTimeout  = 10 * time.Second
	serverIdleConnectionTimeout = 60 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("API stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func run(shutdownSignal context.Context, logger *slog.Logger) error {
	workers, err := demodata.Workers()
	if err != nil {
		return fmt.Errorf("seed workers: %w", err)
	}

	processor, err := disbursement.NewProcessor(
		workers,
		disbursement.ProcessorConfig{
			Provider:                   mockpayment.New(),
			ProviderTimeout:            providerPaymentTimeout,
			ProviderMaxConcurrentCalls: providerMaxConcurrentCalls,
			Logger:                     logger,
		},
	)
	if err != nil {
		return fmt.Errorf("create disbursement processor: %w", err)
	}

	handler, err := httpserver.New(
		processor,
		logger,
		httpserver.Config{
			AllowedOrigin: environmentValue(
				frontendOriginEnvironmentVariable,
				defaultFrontendOrigin,
			),
		},
	)
	if err != nil {
		return fmt.Errorf("create HTTP handler: %w", err)
	}

	server := &http.Server{
		Addr: environmentValue(
			apiAddressEnvironmentVariable,
			defaultAPIAddress,
		),
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverRequestReadTimeout,
		WriteTimeout:      serverResponseWriteTimeout,
		IdleTimeout:       serverIdleConnectionTimeout,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API listening", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-shutdownSignal.Done():
		logger.Info("API shutdown requested")
	}

	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		serverShutdownTimeout,
	)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	processor.Wait()

	logger.Info("API stopped")
	return nil
}

func environmentValue(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
