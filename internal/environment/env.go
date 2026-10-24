package environment

import (
	"fmt"
	"os"
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

	// WorkerDatabaseURL is the reader role the broker hands to a service pod.
	// Empty means the pod is not given a database URL while authentication is on.
	WorkerDatabaseURL = "WORKER_DATABASE_URL"
	// WorkerDBPassword is the password for role foreverbull_worker. It is not
	// the database owner password.
	WorkerDBPassword = "WORKER_DB_PASSWORD"

	NatsURL                   = "NATS_URL"
	NatsURLDefault            = "nats://localhost:4222"
	NatsUser                  = "NATS_USER"
	NatsPassword              = "NATS_PASSWORD"
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

	// AuthEnabled defaults false. Empty is false. Helm and fbenv leave it unset.
	AuthEnabled        = "AUTH_ENABLED"
	AuthEnabledDefault = "false"

	// AuthAdminUsername is the platform admin login. Required only when
	// AUTH_ENABLED is true. Empty means it is not configured.
	AuthAdminUsername = "AUTH_ADMIN_USERNAME"

	// AuthAdminPasswordFile is the path of the admin password file.
	// The password is not an environment variable.
	AuthAdminPasswordFile = "AUTH_ADMIN_PASSWORD_FILE"

	AuthCookieSecure        = "AUTH_COOKIE_SECURE"
	AuthCookieSecureDefault = "false"

	AuthTrustProxy        = "AUTH_TRUST_PROXY"
	AuthTrustProxyDefault = "false"

	// AuthTrustedProxies is a comma-separated CIDR list. It is used only when
	// AUTH_TRUST_PROXY is true. The client address is the rightmost forwarded
	// hop that is not in this list.
	AuthTrustedProxies = "AUTH_TRUSTED_PROXIES"

	// AuthPublicURL is the broker origin browsers use, such as https://fb.example.com.
	// Cookie writes may send this Origin when Sec-Fetch-Site is absent.
	AuthPublicURL = "AUTH_PUBLIC_URL"

	// GitHub OAuth is read only when AUTH_ENABLED is true. All three stay
	// empty by default so a partial set is visible at startup.
	AuthGitHubClientID     = "AUTH_GITHUB_CLIENT_ID"
	AuthGitHubClientSecret = "AUTH_GITHUB_CLIENT_SECRET"
	AuthGitHubRedirectURL  = "AUTH_GITHUB_REDIRECT_URL"
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
	{NatsURL, func() (string, error) { return NatsURLDefault, nil }},
	{NatsDurable, func() (string, error) { return NatsDurableDefault, nil }},
	{NatsDeliveryPolicy, func() (string, error) { return NatsDeliveryPolicyDefault, nil }},
	{MinioURL, func() (string, error) { return MinioURLDefault, nil }},
	{MinioAccessKey, func() (string, error) { return MinioAccessKeyDefault, nil }},
	{MinioSecretKey, func() (string, error) { return MinioSecretKeyDefault, nil }},
	{TradingEnabled, func() (string, error) { return TradingEnabledDefault, nil }},
	{TradingProfilesFile, func() (string, error) { return "", nil }},
	{TradingListingsFile, func() (string, error) { return "", nil }},
	{AuthEnabled, func() (string, error) { return AuthEnabledDefault, nil }},
	{AuthAdminUsername, func() (string, error) { return "", nil }},
	{AuthAdminPasswordFile, func() (string, error) { return "", nil }},
	{AuthCookieSecure, func() (string, error) { return AuthCookieSecureDefault, nil }},
	{AuthTrustProxy, func() (string, error) { return AuthTrustProxyDefault, nil }},
	{AuthGitHubClientID, func() (string, error) { return "", nil }},
	{AuthGitHubClientSecret, func() (string, error) { return "", nil }},
	{AuthGitHubRedirectURL, func() (string, error) { return "", nil }},
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

// GetWorkerDatabaseURL is the URL a service pod may use. A configured reader
// URL wins. With authentication on and no reader URL, the owner URL is not
// returned. With authentication off and no reader URL, the owner URL is used
// so local runs and tests keep a database that has no reader role.
func GetWorkerDatabaseURL() string {
	if url := strings.TrimSpace(os.Getenv(WorkerDatabaseURL)); url != "" {
		return url
	}
	if GetAuthEnabled() {
		return ""
	}
	return GetPostgresURL()
}

func GetNATSURL() string {
	return os.Getenv(NatsURL)
}

func GetNATSUser() string {
	return os.Getenv(NatsUser)
}

func GetNATSPassword() string {
	return os.Getenv(NatsPassword)
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

// GetAuthEnabled reports whether authentication and authorization are on.
// An empty value and a value ParseBool rejects are both false.
func GetAuthEnabled() bool {
	verdict, err := strconv.ParseBool(os.Getenv(AuthEnabled))
	if err != nil {
		return false
	}
	return verdict
}

func GetAuthAdminUsername() string {
	return os.Getenv(AuthAdminUsername)
}

func GetAuthAdminPasswordFile() string {
	return os.Getenv(AuthAdminPasswordFile)
}

func GetAuthCookieSecure() bool {
	verdict, err := strconv.ParseBool(os.Getenv(AuthCookieSecure))
	if err != nil {
		return false
	}
	return verdict
}

func GetAuthTrustProxy() bool {
	verdict, err := strconv.ParseBool(os.Getenv(AuthTrustProxy))
	if err != nil {
		return false
	}
	return verdict
}

func GetAuthTrustedProxies() string {
	return os.Getenv(AuthTrustedProxies)
}

func GetAuthPublicURL() string {
	return os.Getenv(AuthPublicURL)
}

func GetAuthGitHubClientID() string {
	return os.Getenv(AuthGitHubClientID)
}

func GetAuthGitHubClientSecret() string {
	return os.Getenv(AuthGitHubClientSecret)
}

func GetAuthGitHubRedirectURL() string {
	return os.Getenv(AuthGitHubRedirectURL)
}
