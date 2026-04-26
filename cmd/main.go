package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"null-connector/internal/api"
	"null-connector/internal/config"
	"null-connector/internal/grpc"
	"null-connector/internal/provider"
	"null-connector/internal/provider/snaptrade"
	"null-connector/internal/provider/wise"
	"null-connector/internal/runner"

	"github.com/charmbracelet/log"
)

const pollInterval = 10 * time.Second

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

	apiClient, err := api.NewClient(cfg.NullCoreURL, cfg.APIKey)
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

	factory := providerFactory(cfg, apiClient, logger)

	runnerCtx, cancelRunner := context.WithCancel(context.Background())
	defer cancelRunner()
	go runner.New(apiClient, apiClient, factory, logger).Run(runnerCtx, pollInterval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down. bye!")

	cancelRunner()
	grpcHealthSrv.Stop()
}

func providerFactory(cfg config.Config, core *api.Client, logger *log.Logger) runner.ProviderFactory {
	return func(job api.SyncJob) (provider.Provider, error) {
		switch job.Provider {
		case "wise":
			var c wise.Config
			if err := json.Unmarshal(job.Credentials, &c); err != nil {
				return nil, fmt.Errorf("decode wise credentials: %w", err)
			}
			return wise.New(c, core, job.UserID, job.Cursor, logger), nil

		case "snaptrade":
			if cfg.SnapTradeClientID == "" || cfg.SnapTradeConsumerKey == "" {
				return nil, fmt.Errorf("snaptrade app credentials not configured (SNAPTRADE_CLIENT_ID / SNAPTRADE_CONSUMER_KEY)")
			}
			var c snaptrade.Config
			if err := json.Unmarshal(job.Credentials, &c); err != nil {
				return nil, fmt.Errorf("decode snaptrade credentials: %w", err)
			}
			c.ClientID = cfg.SnapTradeClientID
			c.ConsumerKey = cfg.SnapTradeConsumerKey
			return snaptrade.New(c, core, job.UserID, job.Cursor, logger), nil

		default:
			return nil, fmt.Errorf("unknown provider %q", job.Provider)
		}
	}
}
