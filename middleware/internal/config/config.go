package config

import "github.com/kelseyhightower/envconfig"

type GCP struct {
	ProjectID string `envconfig:"GOOGLE_PROJECT_ID"`
}

type DB struct {
	Host     string `envconfig:"DB_HOST"`
	User     string `envconfig:"DB_USER"`
	Password string `envconfig:"DB_PASS"`
	Name     string `envconfig:"DB_NAME"`
}

// Config holds start up config information
type Config struct {
	GCP
	DB
	Env      string `envconfig:"ENV"`
	Port     string `envconfig:"PORT"`
	LogDebug bool   `envconfig:"LOG_DEBUG" default:"false"`
	UseCache string `envconfig:"USE_CACHE" default:"false"`
	// HealthServerPort           string `envconfig:"HEALTH_SERVER_PORT" default:"8080"`
	// GoogleConsumerEnabled      bool   `envconfig:"GOOGLE_CONSUMER_ENABLED" default:"false"`
	// GoogleServiceAccountKey    string `envconfig:"GOOGLE_SERVICE_ACCOUNT_KEY"`
	RedisAddress string `envconfig:"REDIS_ADDRESS" default:"false"`
}

// LoadConfigs loads from environment variables.
// If there are any errors during load it will Panic
func LoadConfigs() Config {
	var cfg Config
	envconfig.MustProcess("", &cfg)
	return cfg
}
