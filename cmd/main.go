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
	"null-connector/internal/provider"
	"null-connector/internal/provider/snaptrade"
	"null-connector/internal/provider/wise"
	"null-connector/internal/runner"

	"github.com/charmbracelet/log"
)

func main() {
	cfg := config.Load()

	logFormatter := log.TextFormatter
	if cfg.LogFormat != "text" {
		logFormatter = log.JSONFormatter
	}
	logger := log.NewWithOptions(io.Writer(os.Stdout), log.Options{
		ReportTimestamp: true,
		Prefix:          "connector",
		Level:           cfg.LogLevel,
		Formatter:       logFormatter,
	})

	logger.Info("starting null-connector")
	logger.Debug("debug is enabled")

	apiClient, err := api.NewClient(cfg.NullCoreURL, cfg.APIKey, cfg.UserID)
	if err != nil {
		logger.Fatal("api client init", "err", err)
	}
	defer func() {
		if err := apiClient.Close(); err != nil {
			logger.Error("close gRPC connection", "err", err)
		}
	}()

	logger.Info("checking null-core connectivity", "url", cfg.NullCoreURL)
	if err := apiClient.Ping(context.Background()); err != nil {
		logger.Fatal("null-core not reachable", "err", err)
	}
	logger.Info("null-core connectivity confirmed")

	providers := buildProviders(cfg, logger)
	for _, p := range providers {
		logger.Info("provider enabled", "name", p.Name())
	}

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

	go runner.New(providers, apiClient, logger).PollAll(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down. bye!")

	grpcHealthSrv.Stop()
}

func buildProviders(cfg config.Config, logger *log.Logger) []provider.Provider {
	var providers []provider.Provider
	if cfg.Wise.Enabled() {
		providers = append(providers, wise.New(wise.Config{
			APIToken:  cfg.Wise.APIToken,
			ProfileID: cfg.Wise.ProfileID,
		}, logger))
	}
	if cfg.SnapTrade.Enabled() {
		providers = append(providers, snaptrade.New(snaptrade.Config{
			ClientID:    cfg.SnapTrade.ClientID,
			ConsumerKey: cfg.SnapTrade.ConsumerKey,
		}, logger))
	}
	return providers
}
