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

	HTTPPort        = "HTTP_PORT"
	HTTPPortDefault = "8080"

	GRPCPort        = "GRPC_PORT"
	GRPCPortDefault = "50055"

	// GrafanaURL is the Grafana origin used in broker-emitted deep links.
	// Kind host-port default is localhost:3000; remote installs override it.
	GrafanaURL        = "GRAFANA_URL"
	GrafanaURLDefault = "http://localhost:3000"

	// DockerNetwork is the Testcontainers network name. The Kubernetes broker
	// runtime does not use it.
	DockerNetwork        = "DOCKER_NETWORK"
	DockerNetworkDefault = "foreverbull"

	BacktestIngestionDefaultName        = "BACKTEST_INGESTION_DEFAULT_NAME"
	BacktestIngestionDefaultNameDefault = "default_ingestion"

	ZiplineContainerName          = "ZIPLINE_CONTAINER_NAME"
	ZiplineContainerNameDefault   = "foreverbull_zipline"
	BacktestPortRangeStart        = "BACKTEST_PORT_RANGE_START"
	BacktestPortRangeStartDefault = "27000"
	BacktestPortRangeEnd          = "BACKTEST_PORT_RANGE_END"
	BacktestPortRangeEndDefault   = "27015"

	LogLevel        = "LOG_LEVEL"
	LogLevelDefault = "warning"

	PostgresURL        = "POSTGRES_URL"
	PostgresURLDefault = "postgres://postgres:foreverbull@localhost:5432/postgres?sslmode=disable"

	NatsURL                   = "NATS_URL"
	NatsURLDefault            = "nats://localhost:4222"
	NatsDurable               = "NATS_DURABLE"
	NatsDurableDefault        = "foreverbull"
	NatsDeliveryPolicy        = "NATS_DELIVERY_POLICY"
	NatsDeliveryPolicyDefault = "all"

	MinioURL              = "MINIO_URL"
	MinioURLDefault       = "localhost:9000"
	MinioAccessKey        = "MINIO_ACCESS_KEY"
	MinioAccessKeyDefault = "minioadmin"
	MinioSecretKey        = "MINIO_SECRET"
	MinioSecretKeyDefault = "minioadmin"

	MarketDataProvider        = "MARKET_DATA_PROVIDER"
	MarketDataProviderDefault = "yahoo"

	TradingEnabled        = "TRADING_ENABLED"
	TradingEnabledDefault = "false"

	TradingProvider        = "TRADING_PROVIDER"
	TradingProviderDefault = "alpaca"

	AlpacaBaseURL        = "ALPACA_MARKETS_BASE_URL"
	AlpacaBaseURLDefault = "https://paper-api.alpaca.markets"
	AlpacaApiKey         = "ALPACA_MARKETS_API_KEY"
	AlpacaApiSecret      = "ALPACA_MARKETS_API_SECRET"

	// TradingCurrency is the account currency. Adapters do not convert FX.
	TradingCurrency        = "TRADING_CURRENCY"
	TradingCurrencyDefault = "USD"

	// TradingAllowLiveURL permits known live broker endpoints and
	// IBKR_ACCOUNT_MODE=live. It does not unlock deploy mode LIVE.
	TradingAllowLiveURL        = "TRADING_ALLOW_LIVE_URL"
	TradingAllowLiveURLDefault = "false"

	// TradingListingsFile replaces the embedded listing sample when set.
	TradingListingsFile = "TRADING_LISTINGS_FILE"

	SaxoBaseURL        = "SAXO_BASE_URL"
	SaxoBaseURLDefault = "https://gateway.saxobank.com/sim/openapi"
	SaxoAccessToken    = "SAXO_ACCESS_TOKEN"
	SaxoAccountKey     = "SAXO_ACCOUNT_KEY"

	IGBaseURL        = "IG_BASE_URL"
	IGBaseURLDefault = "https://demo-api.ig.com/gateway/deal"
	IGAPIKey         = "IG_API_KEY"
	IGIdentifier     = "IG_IDENTIFIER"
	IGPassword       = "IG_PASSWORD"

	IBKRBaseURL            = "IBKR_BASE_URL"
	IBKRBaseURLDefault     = "https://localhost:5000/v1/api"
	IBKRAccountID          = "IBKR_ACCOUNT_ID"
	IBKRAccountMode        = "IBKR_ACCOUNT_MODE"
	IBKRAccountModeDefault = "paper"
	IBKRAccessToken        = "IBKR_ACCESS_TOKEN"
)

type envVar struct {
	name       string
	getDefault func() (string, error)
}

var envVars = []envVar{ //nolint: gochecknoglobals
	{ServerAddress, func() (string, error) { return ServerAddressDefault, nil }},
	{HTTPPort, func() (string, error) { return HTTPPortDefault, nil }},
	{GRPCPort, func() (string, error) { return GRPCPortDefault, nil }},
	{GrafanaURL, func() (string, error) { return GrafanaURLDefault, nil }},
	{BacktestIngestionDefaultName, func() (string, error) { return BacktestIngestionDefaultNameDefault, nil }},
	{ZiplineContainerName, func() (string, error) { return ZiplineContainerNameDefault, nil }},
	{BacktestPortRangeStart, func() (string, error) { return BacktestPortRangeStartDefault, nil }},
	{BacktestPortRangeEnd, func() (string, error) { return BacktestPortRangeEndDefault, nil }},
	{LogLevel, func() (string, error) { return LogLevelDefault, nil }},
	{DockerNetwork, func() (string, error) { return DockerNetworkDefault, nil }},
	{PostgresURL, func() (string, error) { return PostgresURLDefault, nil }},
	{NatsURL, func() (string, error) { return NatsURLDefault, nil }},
	{NatsDurable, func() (string, error) { return NatsDurableDefault, nil }},
	{NatsDeliveryPolicy, func() (string, error) { return NatsDeliveryPolicyDefault, nil }},
	{MinioURL, func() (string, error) { return MinioURLDefault, nil }},
	{MinioAccessKey, func() (string, error) { return MinioAccessKeyDefault, nil }},
	{MinioSecretKey, func() (string, error) { return MinioSecretKeyDefault, nil }},
	{MarketDataProvider, func() (string, error) { return MarketDataProviderDefault, nil }},
	{TradingEnabled, func() (string, error) { return TradingEnabledDefault, nil }},
	{TradingProvider, func() (string, error) { return TradingProviderDefault, nil }},
	{AlpacaBaseURL, func() (string, error) { return AlpacaBaseURLDefault, nil }},
	{AlpacaApiKey, func() (string, error) { return "", nil }},
	{AlpacaApiSecret, func() (string, error) { return "", nil }},
	{TradingCurrency, func() (string, error) { return TradingCurrencyDefault, nil }},
	{TradingAllowLiveURL, func() (string, error) { return TradingAllowLiveURLDefault, nil }},
	{TradingListingsFile, func() (string, error) { return "", nil }},
	{SaxoBaseURL, func() (string, error) { return SaxoBaseURLDefault, nil }},
	{SaxoAccessToken, func() (string, error) { return "", nil }},
	{SaxoAccountKey, func() (string, error) { return "", nil }},
	{IGBaseURL, func() (string, error) { return IGBaseURLDefault, nil }},
	{IGAPIKey, func() (string, error) { return "", nil }},
	{IGIdentifier, func() (string, error) { return "", nil }},
	{IGPassword, func() (string, error) { return "", nil }},
	{IBKRBaseURL, func() (string, error) { return IBKRBaseURLDefault, nil }},
	{IBKRAccountID, func() (string, error) { return "", nil }},
	{IBKRAccountMode, func() (string, error) { return IBKRAccountModeDefault, nil }},
	{IBKRAccessToken, func() (string, error) { return "", nil }},
}

type envSetter interface {
	Helper()
	Setenv(key, value string)
	Fatalf(format string, args ...any)
}

func applyEnvDefaults(setenv func(name, value string) error) error {
	for _, envVar := range envVars {
		if os.Getenv(envVar.name) == "" {
			defaultEnv, err := envVar.getDefault()
			if err != nil {
				return fmt.Errorf("failed to set default value for %s: %w", envVar.name, err)
			}

			if err := setenv(envVar.name, defaultEnv); err != nil {
				return fmt.Errorf("failed to set default value for %s: %w", envVar.name, err)
			}
		}
	}

	configureLogging()

	return nil
}

func configureLogging() {
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
}

// Setup fills empty process env vars with package defaults and configures logging.
// It does not start Docker or any other infrastructure.
func Setup() error {
	return applyEnvDefaults(os.Setenv)
}

// SetupForTest applies the same empty-var defaults as Setup, using t.Setenv so
// values are restored when the test ends. It does not start Docker or other
// infrastructure. t is typically *testing.T.
func SetupForTest(t envSetter) {
	t.Helper()

	if err := applyEnvDefaults(func(name, value string) error {
		t.Setenv(name, value)
		return nil
	}); err != nil {
		t.Fatalf("environment.SetupForTest: %v", err)
	}
}

func GetServerAddress() string {
	return os.Getenv(ServerAddress)
}

func GetHTTPPort() string {
	return os.Getenv(HTTPPort)
}

func GetGRPCPort() string {
	return os.Getenv(GRPCPort)
}

func GetGrafanaURL() string {
	return os.Getenv(GrafanaURL)
}

func GetServerGRPCURL() string {
	return fmt.Sprintf("localhost:%s", GetGRPCPort())
}

func GetDockerNetworkName() string {
	return os.Getenv(DockerNetwork)
}

func GetBacktestIngestionDefaultName() string {
	return os.Getenv(BacktestIngestionDefaultName)
}

func GetZiplineContainerName() string {
	return os.Getenv(ZiplineContainerName)
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
	return os.Getenv(PostgresURL)
}

func GetNATSURL() string {
	return os.Getenv(NatsURL)
}

func GetNATSDurable() string {
	return os.Getenv(NatsDurable)
}

func GetNATSDeliveryPolicy() string {
	return os.Getenv(NatsDeliveryPolicy)
}

func GetMinioURL() string {
	return os.Getenv(MinioURL)
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

func GetTradingEnabled() bool {
	verdict, err := strconv.ParseBool(os.Getenv(TradingEnabled))
	if err != nil {
		return false
	}
	return verdict
}

func GetTradingProvider() string {
	return os.Getenv(TradingProvider)
}

func GetAlpacaBaseURL() string {
	return os.Getenv(AlpacaBaseURL)
}

func GetAlpacaAPIKey() string {
	return os.Getenv(AlpacaApiKey)
}

func GetAlpacaAPISecret() string {
	return os.Getenv(AlpacaApiSecret)
}

func GetTradingCurrency() string {
	return os.Getenv(TradingCurrency)
}

func GetTradingAllowLiveURL() bool {
	verdict, err := strconv.ParseBool(os.Getenv(TradingAllowLiveURL))
	if err != nil {
		return false
	}
	return verdict
}

func GetTradingListingsFile() string {
	return os.Getenv(TradingListingsFile)
}

func GetSaxoBaseURL() string {
	return os.Getenv(SaxoBaseURL)
}

func GetSaxoAccessToken() string {
	return os.Getenv(SaxoAccessToken)
}

func GetSaxoAccountKey() string {
	return os.Getenv(SaxoAccountKey)
}

func GetIGBaseURL() string {
	return os.Getenv(IGBaseURL)
}

func GetIGAPIKey() string {
	return os.Getenv(IGAPIKey)
}

func GetIGIdentifier() string {
	return os.Getenv(IGIdentifier)
}

func GetIGPassword() string {
	return os.Getenv(IGPassword)
}

func GetIBKRBaseURL() string {
	return os.Getenv(IBKRBaseURL)
}

func GetIBKRAccountID() string {
	return os.Getenv(IBKRAccountID)
}

func GetIBKRAccountMode() string {
	return os.Getenv(IBKRAccountMode)
}

func GetIBKRAccessToken() string {
	return os.Getenv(IBKRAccessToken)
}
