package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"null-connector/internal/api"
	"null-connector/internal/config"
	"null-connector/internal/grpc"

	"github.com/charmbracelet/log"
)

func main() {
	cfg := config.Load()

	logWriter := io.Writer(os.Stdout)
	logFormatter := log.TextFormatter
	if cfg.LogFormat != "text" {
		logFormatter = log.JSONFormatter
	}

	logger := log.NewWithOptions(logWriter, log.Options{
		ReportTimestamp: true,
		Prefix:          "connector",
		Level:           cfg.LogLevel,
		Formatter:       logFormatter,
	})

	logger.Info("starting null-connector")
	logger.Debug("debug is enabled")

	apiClient, err := api.NewClient(cfg.NullCoreURL, cfg.APIKey)
	if err != nil {
		logger.Fatal("api client init", "err", err)
	}
	defer func() {
		if err := apiClient.Close(); err != nil {
			logger.Error("failed to close gRPC connection", "err", err)
		}
	}()

	logger.Info("checking null-core connectivity", "url", cfg.NullCoreURL)
	if err := apiClient.Ping(context.Background()); err != nil {
		logger.Fatal("null-core not reachable", "err", err)
	}
	logger.Info("null-core connectivity confirmed")

	grpcHealthSrv, err := grpc.NewHealthServer(cfg.GRPCAddress)
	if err != nil {
		logger.Fatal("grpc health server init", "err", err)
	}

	go func() {
		logger.Info("grpc health server starting", "address", cfg.GRPCAddress)
		if err := grpcHealthSrv.Start(); err != nil {
			logger.Error("grpc health server error", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down. bye!")

	grpcHealthSrv.Stop()
}
