package main

import (
	"io"
	"os"
	"os/signal"
	"syscall"

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
