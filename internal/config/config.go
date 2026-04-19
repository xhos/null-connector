package config

import (
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

type Config struct {
	NullCoreURL string
	APIKey      string
	UserID      string

	GRPCAddress string

	Wise      WiseSettings
	SnapTrade SnapTradeSettings

	LogLevel  log.Level
	LogFormat string
}

type WiseSettings struct {
	APIToken  string
	ProfileID string
}

func (s WiseSettings) Enabled() bool { return s.APIToken != "" }

type SnapTradeSettings struct {
	ClientID    string
	ConsumerKey string
}

func (s SnapTradeSettings) Enabled() bool { return s.ClientID != "" && s.ConsumerKey != "" }

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

	userID := os.Getenv("NULL_USER_ID")
	if userID == "" {
		panic("NULL_USER_ID environment variable is required")
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
		NullCoreURL: nullCoreURL,
		APIKey:      apiKey,
		UserID:      userID,
		GRPCAddress: parseAddress(grpcAddress),
		Wise: WiseSettings{
			APIToken:  os.Getenv("WISE_API_TOKEN"),
			ProfileID: os.Getenv("WISE_PROFILE_ID"),
		},
		SnapTrade: SnapTradeSettings{
			ClientID:    os.Getenv("SNAPTRADE_CLIENT_ID"),
			ConsumerKey: os.Getenv("SNAPTRADE_CONSUMER_KEY"),
		},
		LogLevel:  logLevel,
		LogFormat: logFormat,
	}
}
