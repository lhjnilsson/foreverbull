package environment

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	ServerAddress        = "SERVER_ADDRESS"
	ServerAddressDefault = "foreverbull"

	HttpPort        = "HTTP_PORT"
	HttpPortDefault = "8080"

	DockerNetwork        = "DOCKER_NETWORK"
	DockerNetworkDefault = "foreverbull"

	BacktestIngestionDefaultName        = "BACKTEST_INGESTION_DEFAULT_NAME"
	BacktestIngestionDefaultNameDefault = "default_ingestion"

	BacktestImage                 = "BACKTEST_IMAGE"
	BacktestImageDefault          = "lhjnilson/zipline:latest"
	BacktestPortRangeStart        = "BACKTEST_PORT_RANGE_START"
	BacktestPortRangeStartDefault = "27000"
	BacktestPortRangeEnd          = "BACKTEST_PORT_RANGE_END"
	BacktestPortRangeEndDefault   = "27015"

	LogLevel        = "LOG_LEVEL"
	LogLevelDefault = "warning"

	PostgresUrl        = "POSTGRES_URL"
	PostgresUrlDefault = "postgres://postgres:foreverbull@localhost:5432/postgres?sslmode=disable"

	NatsUrl                   = "NATS_URL"
	NatsUrlDefault            = "nats://localhost:4222"
	NatsDurable               = "NATS_DURABLE"
	NatsDurableDefault        = "foreverbull"
	NatsDeliveryPolicy        = "NATS_DELIVERY_POLICY"
	NatsDeliveryPolicyDefault = "all"

	MinioUrl              = "MINIO_URL"
	MinioUrlDefault       = "localhost:9000"
	MinioAccessKey        = "MINIO_ACCESS_KEY"
	MinioAccessKeyDefault = "minioadmin"
	MinioSecretKey        = "MINIO_SECRET"
	MinioSecretKeyDefault = "minioadmin"

	MarketDataProvider        = "MARKET_DATA_PROVIDER"
	MarketDataProviderDefault = "alpaca_markets"
	AlpacaBaseUrl             = "ALPACA_MARKETS_BASE_URL"
	AlpacaBaseUrlDefault      = "https://paper-api.alpaca.markets"
	AlpacaApiKey              = "ALPACA_MARKETS_API_KEY"
	AlpacaApiSecret           = "ALPACA_MARKETS_API_SECRET"
)

type envVar struct {
	name       string
	getDefault func() (string, error)
}

var envVars = []envVar{ //nolint: gochecknoglobals
	{ServerAddress, func() (string, error) { return ServerAddressDefault, nil }},
	{HttpPort, func() (string, error) { return HttpPortDefault, nil }},
	{BacktestIngestionDefaultName, func() (string, error) { return BacktestIngestionDefaultNameDefault, nil }},
	{BacktestImage, func() (string, error) { return BacktestImageDefault, nil }},
	{BacktestPortRangeStart, func() (string, error) { return BacktestPortRangeStartDefault, nil }},
	{BacktestPortRangeEnd, func() (string, error) { return BacktestPortRangeEndDefault, nil }},
	{LogLevel, func() (string, error) { return LogLevelDefault, nil }},
	{DockerNetwork, func() (string, error) { return DockerNetworkDefault, nil }},
	{PostgresUrl, func() (string, error) { return PostgresUrlDefault, nil }},
	{NatsUrl, func() (string, error) { return NatsUrlDefault, nil }},
	{NatsDurable, func() (string, error) { return NatsDurableDefault, nil }},
	{NatsDeliveryPolicy, func() (string, error) { return NatsDeliveryPolicyDefault, nil }},
	{MinioUrl, func() (string, error) { return MinioUrlDefault, nil }},
	{MinioAccessKey, func() (string, error) { return MinioAccessKeyDefault, nil }},
	{MinioSecretKey, func() (string, error) { return MinioSecretKeyDefault, nil }},
	{MarketDataProvider, func() (string, error) { return MarketDataProviderDefault, nil }},
	{AlpacaBaseUrl, func() (string, error) { return AlpacaBaseUrlDefault, nil }},
	{AlpacaApiKey, func() (string, error) { return "", nil }},
	{AlpacaApiSecret, func() (string, error) { return "", nil }},
}

func Setup() error {
	for _, envVar := range envVars {
		if os.Getenv(envVar.name) == "" {
			defaultEnv, err := envVar.getDefault()
			if err != nil {
				return fmt.Errorf("failed to set default value for %s: %w", envVar.name, err)
			}

			if err := os.Setenv(envVar.name, defaultEnv); err != nil {
				return fmt.Errorf("failed to set default value for %s: %w", envVar.name, err)
			}
		}
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	switch os.Getenv(LogLevel) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		log.Warn().Msgf("unknown log level: %s", os.Getenv(LogLevel))
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}

	return nil
}

func GetServerAddress() string {
	return os.Getenv(ServerAddress)
}

func GetHTTPPort() string {
	return os.Getenv(HttpPort)
}

func GetDockerNetworkName() string {
	return os.Getenv(DockerNetwork)
}

func GetBacktestIngestionDefaultName() string {
	return os.Getenv(BacktestIngestionDefaultName)
}

func GetBacktestImage() string {
	return os.Getenv(BacktestImage)
}

func GetBacktestPortRangeStart() int {
	portStr := os.Getenv(BacktestPortRangeStart)

	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Errorf("failed to convert BACKTEST_PORT_RANGE_START to int: %w", err))
	}

	return port
}

func GetBacktestPortRangeEnd() int {
	portStr := os.Getenv(BacktestPortRangeEnd)

	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Errorf("failed to convert BACKTEST_PORT_RANGE_END to int: %w", err))
	}

	return port
}

func GetLogLevel() string {
	return os.Getenv(LogLevel)
}

func GetPostgresURL() string {
	return os.Getenv(PostgresUrl)
}

func GetNATSURL() string {
	return os.Getenv(NatsUrl)
}

func GetNATSDurable() string {
	return os.Getenv(NatsDurable)
}

func GetNATSDeliveryPolicy() string {
	return os.Getenv(NatsDeliveryPolicy)
}

func GetMinioURL() string {
	return os.Getenv(MinioUrl)
}

func GetMinioAccessKey() string {
	return os.Getenv(MinioAccessKey)
}

func GetMinioSecretKey() string {
	return os.Getenv(MinioSecretKey)
}

func GetMarketDataProvider() string {
	return os.Getenv(MarketDataProvider)
}

func GetAlpacaBaseURL() string {
	return os.Getenv(AlpacaBaseUrl)
}

func GetAlpacaAPIKey() string {
	return os.Getenv(AlpacaApiKey)
}

func GetAlpacaAPISecret() string {
	return os.Getenv(AlpacaApiSecret)
}
