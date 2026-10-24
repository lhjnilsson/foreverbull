package environment

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

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

	// WorkerPostgresPassword is the login password for role foreverbull_worker.
	// Helm stores it in Secret foreverbull-credentials. It is never the broker
	// database password, and service pods do not receive it.
	WorkerPostgresPassword        = "WORKER_POSTGRES_PASSWORD"
	WorkerPostgresPasswordDefault = "foreverbull-worker"
	// WorkerPostgresRole is the login that strategy workers use to read bars.
	WorkerPostgresRole = "foreverbull_worker"

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

	TradingEnabled        = "TRADING_ENABLED"
	TradingEnabledDefault = "false"

	// TradingProfilesFile is the path of the trading profiles YAML.
	// Credentials live in that file. Empty means the file is not configured.
	TradingProfilesFile = "TRADING_PROFILES_FILE"

	// TradingListingsFile replaces the embedded listing sample when a profile
	// does not set its own listings path.
	TradingListingsFile = "TRADING_LISTINGS_FILE"
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
	{PostgresURL, func() (string, error) { return PostgresURLDefault, nil }},
	{WorkerPostgresPassword, func() (string, error) { return WorkerPostgresPasswordDefault, nil }},
	{NatsURL, func() (string, error) { return NatsURLDefault, nil }},
	{NatsDurable, func() (string, error) { return NatsDurableDefault, nil }},
	{NatsDeliveryPolicy, func() (string, error) { return NatsDeliveryPolicyDefault, nil }},
	{MinioURL, func() (string, error) { return MinioURLDefault, nil }},
	{MinioAccessKey, func() (string, error) { return MinioAccessKeyDefault, nil }},
	{MinioSecretKey, func() (string, error) { return MinioSecretKeyDefault, nil }},
	{TradingEnabled, func() (string, error) { return TradingEnabledDefault, nil }},
	{TradingProfilesFile, func() (string, error) { return "", nil }},
	{TradingListingsFile, func() (string, error) { return "", nil }},
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

	if _, err := GetBacktestPortRange(); err != nil {
		return err
	}

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

func GetBacktestIngestionDefaultName() string {
	return os.Getenv(BacktestIngestionDefaultName)
}

func GetZiplineContainerName() string {
	return os.Getenv(ZiplineContainerName)
}

// PortRange is an inclusive range of TCP ports: Start and End can both be used.
type PortRange struct {
	Start int
	End   int
}

// Contains reports whether port is inside the inclusive range.
func (r PortRange) Contains(port int) bool {
	return port >= r.Start && port <= r.End
}

// GetBacktestPortRange reads BACKTEST_PORT_RANGE_START and BACKTEST_PORT_RANGE_END.
// Both ends are inclusive. A value that is not a port, or a start above the end,
// is an error that names the variable, so a bad setting fails at startup
// instead of on the first session.
func GetBacktestPortRange() (PortRange, error) {
	start, err := parsePort(BacktestPortRangeStart)
	if err != nil {
		return PortRange{}, err
	}

	end, err := parsePort(BacktestPortRangeEnd)
	if err != nil {
		return PortRange{}, err
	}

	if start > end {
		return PortRange{}, fmt.Errorf(
			"%s (%d) is above %s (%d); set the start to the lower port",
			BacktestPortRangeStart, start, BacktestPortRangeEnd, end,
		)
	}

	return PortRange{Start: start, End: end}, nil
}

func parsePort(name string) (int, error) {
	value := os.Getenv(name)

	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a port number; set it to a value from 1 to 65535", name, value)
	}

	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s=%d is outside 1-65535; set it to a valid TCP port", name, port)
	}

	return port, nil
}

func GetLogLevel() string {
	return os.Getenv(LogLevel)
}

func GetPostgresURL() string {
	return os.Getenv(PostgresURL)
}

// workerPasswordPattern matches the chart's URL-safe credential rule.
var workerPasswordPattern = regexp.MustCompile(`^[-A-Za-z0-9._~]+$`)

func workerPostgresPassword() (string, error) {
	password := strings.TrimSpace(os.Getenv(WorkerPostgresPassword))
	if password == "" {
		password = WorkerPostgresPasswordDefault
	}
	if !workerPasswordPattern.MatchString(password) {
		return "", fmt.Errorf("%s must be URL-safe (letters, digits and -._~)", WorkerPostgresPassword)
	}

	return password, nil
}

// WorkerDatabaseURL is the read-only DSN strategy workers use for bar reads.
// The user is foreverbull_worker. The broker database user and password are
// not copied into it.
func WorkerDatabaseURL() (string, error) {
	raw := strings.TrimSpace(GetPostgresURL())
	if raw == "" {
		return "", fmt.Errorf("%s is empty", PostgresURL)
	}

	password, err := workerPostgresPassword()
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", PostgresURL, err)
	}
	if parsed.User == nil || parsed.User.Username() == "" {
		return "", fmt.Errorf("%s has no user", PostgresURL)
	}
	if parsed.User.Username() == WorkerPostgresRole {
		return "", fmt.Errorf("%s user is %s; the broker must connect as the database owner", PostgresURL, WorkerPostgresRole)
	}
	if super, ok := parsed.User.Password(); ok && super == password {
		return "", fmt.Errorf("%s matches the broker database password", WorkerPostgresPassword)
	}

	parsed.User = url.UserPassword(WorkerPostgresRole, password)

	return parsed.String(), nil
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

func GetTradingEnabled() bool {
	verdict, err := strconv.ParseBool(os.Getenv(TradingEnabled))
	if err != nil {
		return false
	}
	return verdict
}

func GetTradingProfilesFile() string {
	return os.Getenv(TradingProfilesFile)
}

func GetTradingListingsFile() string {
	return os.Getenv(TradingListingsFile)
}
