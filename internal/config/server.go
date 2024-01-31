package config

import (
	"github.com/spf13/viper"
)

type HTTP struct {
	UIStaticPath string `yaml:"ui_static_path" json:"ui_static_path"`
	Port         int    `yaml:"port" json:"port"`
}

type Docker struct {
	Network string `yaml:"network" json:"network"`
}

type ExposedPortRange struct {
	Start int `yaml:"start" json:"start"`
	End   int `yaml:"end" json:"end"`
}

type AlpacaMarkets struct {
	BaseURL   string `yaml:"base_url" json:"base_url"`
	APIKey    string `yaml:"api_key" json:"api_key"`
	APISecret string `yaml:"api_secret" json:"api_secret"`
}

type Provider struct {
	Alpaca AlpacaMarkets `yaml:"alpaca_markets" json:"alpaca_markets"`
}

type Config struct {
	Hostname             string           `yaml:"hostname" json:"hostname"`
	ClientLogLevel       string           `yaml:"client_log_level" json:"client_log_level"`
	HTTP                 HTTP             `yaml:"http" json:"http"`
	Docker               Docker           `yaml:"docker" json:"docker"`
	ExposedPortRange     ExposedPortRange `yaml:"exposed_port_range" json:"exposed_port_range"`
	PostgresURI          string           `yaml:"postgres_uri" json:"postgres_uri"`
	NATSURI              string           `yaml:"nats_uri" json:"nats_uri"`
	NATS_DURABLE         string           `yaml:"nats_durable" json:"nats_durable"`
	NATS_DELIVERY_POLICY string           `yaml:"nats_delivery_policy" json:"nats_delivery_policy"`
	MinioURI             string           `yaml:"minio_uri" json:"minio_uri"`
	MinioAccessKey       string
	MinioSecretKey       string
	Provider             Provider `yaml:"provider" json:"provider"`
}

func GetConfig() (*Config, error) {
	viper.SetDefault("HOSTNAME", "foreverbull")
	viper.SetDefault("CLIENT_LOG_LEVEL", "warning")
	viper.SetDefault("UI_STATIC_PATH", "./external/ui/dist")
	viper.SetDefault("HTTP_PORT", 8080)
	viper.SetDefault("DOCKER_NETWORK", "fb-internal")
	viper.SetDefault("DATABASE_NETLOC", "127.0.0.1")
	viper.SetDefault("EXPOSED_PORT_RANGE_START", 27000)
	viper.SetDefault("EXPOSED_PORT_RANGE_END", 27015)

	viper.SetDefault("POSTGRES_URI", "postgres://postgres:foreverbull@localhost:5432/postgres?sslmode=disable")

	viper.SetDefault("NATS_URI", "nats://localhost:4222")
	viper.SetDefault("NATS_DURABLE", "foreverbull")
	viper.SetDefault("NATS_DELIVERY_POLICY", "all")

	viper.SetDefault("MINIO_URI", "http://localhost:9000")
	viper.SetDefault("MINIO_ACCESS_KEY", "minioadmin")
	viper.SetDefault("MINIO_SECRET_KEY", "minioadmin")

	consumerName := "foreverbull"
	viper.SetDefault("NATS_CONSUMER_NAME", consumerName)

	viper.SetDefault("STORAGE_ENDPOINT", "127.0.0.1:9000")
	viper.SetDefault("STORAGE_ACCESS_KEY", "minioadmin")
	viper.SetDefault("STORAGE_SECRET_KEY", "minioadmin")
	viper.SetDefault("BACKTEST_MAX_CONCURRENT", 2)
	viper.SetDefault("BACKTEST_POLLING_INTERVAL", 1)

	viper.SetConfigType("env")
	viper.SetConfigName(".env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		// Don't care if .env file is not found
		_, notFoundError := err.(viper.ConfigFileNotFoundError)
		if !notFoundError {
			return nil, err
		}
	}
	viper.AutomaticEnv()

	config := Config{
		Hostname:       viper.GetString("HOSTNAME"),
		ClientLogLevel: viper.GetString("CLIENT_LOG_LEVEL"),
		HTTP: HTTP{
			UIStaticPath: viper.GetString("UI_STATIC_PATH"),
			Port:         viper.GetInt("HTTP_PORT"),
		},
		Docker: Docker{
			Network: viper.GetString("DOCKER_NETWORK"),
		},
		ExposedPortRange: ExposedPortRange{
			Start: viper.GetInt("EXPOSED_PORT_RANGE_START"),
			End:   viper.GetInt("EXPOSED_PORT_RANGE_END"),
		},
		Provider: Provider{
			Alpaca: AlpacaMarkets{
				BaseURL:   viper.GetString("ALPACA_MARKETS_BASE_URL"),
				APIKey:    viper.GetString("ALPACA_MARKETS_API_KEY"),
				APISecret: viper.GetString("ALPACA_MARKETS_API_SECRET"),
			},
		},
	}
	return &config, nil
}
