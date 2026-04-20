package config

import (
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

type Config struct {
	NullCoreURL string
	APIKey      string

	GRPCAddress string

	// optional
	SnapTradeClientID    string
	SnapTradeConsumerKey string

	LogLevel  log.Level
	LogFormat string
}

func parseAddress(port string) string {
	port = strings.TrimSpace(port)
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}

func Load() Config {
	nullCoreURL := os.Getenv("NULL_CORE_URL")
	if nullCoreURL == "" {
		panic("NULL_CORE_URL environment variable is required")
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		panic("API_KEY environment variable is required")
	}

	grpcAddress := os.Getenv("GRPC_PORT")
	if grpcAddress == "" {
		grpcAddress = "127.0.0.1:55558"
	}

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}

	logFormat := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if logFormat != "json" && logFormat != "text" {
		logFormat = "text"
	}

	return Config{
		NullCoreURL:          nullCoreURL,
		APIKey:               apiKey,
		GRPCAddress:          parseAddress(grpcAddress),
		SnapTradeClientID:    os.Getenv("SNAPTRADE_CLIENT_ID"),
		SnapTradeConsumerKey: os.Getenv("SNAPTRADE_CONSUMER_KEY"),
		LogLevel:             logLevel,
		LogFormat:            logFormat,
	}
}
