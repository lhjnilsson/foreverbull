package environment

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	SERVER_ADDRESS         = "SERVER_ADDRESS"
	SERVER_ADDRESS_DEFAULT = "foreverbull"

	HTTP_PORT         = "HTTP_PORT"
	HTTP_PORT_DEFAULT = "8080"

	UI_STATIC_PATH         = "UI_STATIC_PATH"
	UI_STATIC_PATH_DEFAULT = "./external/ui/dist"

	DOCKER_NETWORK         = "DOCKER_NETWORK"
	DOCKER_NETWORK_DEFAULT = "foreverbull"

	BACKTEST_IMAGE                    = "BACKTEST_IMAGE"
	BACKTEST_IMAGE_DEFAULT            = "lhjnilson/zipline:latest"
	BACKTEST_PORT_RANGE_START         = "BACKTEST_PORT_RANGE_START"
	BACKTEST_PORT_RANGE_START_DEFAULT = "27000"
	BACKTEST_PORT_RANGE_END           = "BACKTEST_PORT_RANGE_END"
	BACKTEST_PORT_RANGE_END_DEFAULT   = "27015"

	LOG_LEVEL         = "LOG_LEVEL"
	LOG_LEVEL_DEFAULT = "warning"

	POSTGRES_URL         = "POSTGRES_URL"
	POSTGRES_URL_DEFAULT = "postgres://postgres:foreverbull@localhost:5432/postgres?sslmode=disable"

	NATS_URL                     = "NATS_URL"
	NATS_URL_DEFAULT             = "nats://localhost:4222"
	NATS_DURABLE                 = "NATS_DURABLE"
	NATS_DURABLE_DEFAULT         = "foreverbull"
	NATS_DELIVERY_POLICY         = "NATS_DELIVERY_POLICY"
	NATS_DELIVERY_POLICY_DEFAULT = "all"

	MINIO_URL                = "MINIO_URL"
	MINIO_URL_DEFAULT        = "localhost:9000"
	MINIO_ACCESS_KEY         = "MINIO_ACCESS_KEY"
	MINIO_ACCESS_KEY_DEFAULT = "minioadmin"
	MINIO_SECRET_KEY         = "MINIO_SECRET"
	MINIO_SECRET_KEY_DEFAULT = "minioadmin"

	MARKET_DATA_PROVIDER         = "MARKET_DATA_PROVIDER"
	MARKET_DATA_PROVIDER_DEFAULT = "alpaca_markets"
	ALPACA_BASE_URL              = "ALPACA_MARKETS_BASE_URL"
	ALPACA_BASE_URL_DEFAULT      = "https://paper-api.alpaca.markets"
	ALPACA_API_KEY               = "ALPACA_MARKETS_API_KEY"
	ALPACA_API_SECRET            = "ALPACA_MARKETS_API_SECRET"
)

type envVar struct {
	name       string
	getDefault func() (string, error)
}

var envVars = []envVar{
	{SERVER_ADDRESS, func() (string, error) { return SERVER_ADDRESS_DEFAULT, nil }},
	{HTTP_PORT, func() (string, error) { return HTTP_PORT_DEFAULT, nil }},
	{BACKTEST_IMAGE, func() (string, error) { return BACKTEST_IMAGE_DEFAULT, nil }},
	{BACKTEST_PORT_RANGE_START, func() (string, error) { return BACKTEST_PORT_RANGE_START_DEFAULT, nil }},
	{BACKTEST_PORT_RANGE_END, func() (string, error) { return BACKTEST_PORT_RANGE_END_DEFAULT, nil }},
	{LOG_LEVEL, func() (string, error) { return LOG_LEVEL_DEFAULT, nil }},
	{UI_STATIC_PATH, func() (string, error) { return UI_STATIC_PATH_DEFAULT, nil }},
	{DOCKER_NETWORK, func() (string, error) { return DOCKER_NETWORK_DEFAULT, nil }},
	{POSTGRES_URL, func() (string, error) { return POSTGRES_URL_DEFAULT, nil }},
	{NATS_URL, func() (string, error) { return NATS_URL_DEFAULT, nil }},
	{NATS_DURABLE, func() (string, error) { return NATS_DURABLE_DEFAULT, nil }},
	{NATS_DELIVERY_POLICY, func() (string, error) { return NATS_DELIVERY_POLICY_DEFAULT, nil }},
	{MINIO_URL, func() (string, error) { return MINIO_URL_DEFAULT, nil }},
	{MINIO_ACCESS_KEY, func() (string, error) { return MINIO_ACCESS_KEY_DEFAULT, nil }},
	{MINIO_SECRET_KEY, func() (string, error) { return MINIO_SECRET_KEY_DEFAULT, nil }},
	{MARKET_DATA_PROVIDER, func() (string, error) { return MARKET_DATA_PROVIDER_DEFAULT, nil }},
	{ALPACA_BASE_URL, func() (string, error) { return ALPACA_BASE_URL_DEFAULT, nil }},
	{ALPACA_API_KEY, func() (string, error) { return "", fmt.Errorf("ALPACA_API_KEY is required") }},
	{ALPACA_API_SECRET, func() (string, error) { return "", fmt.Errorf("ALPACA_API_SECRET is required") }},
}

func Setup() error {
	for _, v := range envVars {
		if os.Getenv(v.name) == "" {
			defaultEnv, err := v.getDefault()
			if err != nil {
				return fmt.Errorf("failed to set default value for %s: %w", v.name, err)
			}
			if err := os.Setenv(v.name, defaultEnv); err != nil {
				return fmt.Errorf("failed to set default value for %s: %w", v.name, err)
			}
		}
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	switch os.Getenv(LOG_LEVEL) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		log.Warn().Msgf("unknown log level: %s", os.Getenv(LOG_LEVEL))
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}
	return nil
}

func GetServerAddress() string {
	return os.Getenv(SERVER_ADDRESS)
}

func GetHTTPPort() string {
	return os.Getenv(HTTP_PORT)
}

func GetUIStaticPath() string {
	return os.Getenv(UI_STATIC_PATH)
}

func GetDockerNetworkName() string {
	return os.Getenv(DOCKER_NETWORK)
}

func GetBacktestImage() string {
	return os.Getenv(BACKTEST_IMAGE)
}

func GetBacktestPortRangeStart() int {
	portStr := os.Getenv(BACKTEST_PORT_RANGE_START)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Errorf("failed to convert BACKTEST_PORT_RANGE_START to int: %w", err))
	}
	return port
}

func GetBacktestPortRangeEnd() int {
	portStr := os.Getenv(BACKTEST_PORT_RANGE_END)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Errorf("failed to convert BACKTEST_PORT_RANGE_END to int: %w", err))
	}
	return port
}

func GetLogLevel() string {
	return os.Getenv(LOG_LEVEL)
}

func GetPostgresURL() string {
	return os.Getenv(POSTGRES_URL)
}

func GetNATSURL() string {
	return os.Getenv(NATS_URL)
}

func GetNATSDurable() string {
	return os.Getenv(NATS_DURABLE)
}

func GetNATSDeliveryPolicy() string {
	return os.Getenv(NATS_DELIVERY_POLICY)
}

func GetMinioURL() string {
	return os.Getenv(MINIO_URL)
}

func GetMinioAccessKey() string {
	return os.Getenv(MINIO_ACCESS_KEY)
}

func GetMinioSecretKey() string {
	return os.Getenv(MINIO_SECRET_KEY)
}

func GetMarketDataProvider() string {
	return os.Getenv(MARKET_DATA_PROVIDER)
}

func GetAlpacaBaseURL() string {
	return os.Getenv(ALPACA_BASE_URL)
}

func GetAlpacaAPIKey() string {
	return os.Getenv(ALPACA_API_KEY)
}

func GetAlpacaAPISecret() string {
	return os.Getenv(ALPACA_API_SECRET)
}
