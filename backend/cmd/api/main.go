package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &http.Server{
		Addr:              ":8080",
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("API listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}
