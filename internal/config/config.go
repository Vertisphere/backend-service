package config

import "github.com/kelseyhightower/envconfig"

// PubSub contains the config params required for working with Google PubSub
type PubSub struct {
	MaxMessages int `envconfig:"PUBSUB_MAX_MESSAGES" default:"10"`
}

// Subscriptions hold information for PubSub subscriptions
type Subscriptions struct {
	HomeEventsEnabled     bool   `envconfig:"SUBSCRIPTION_HOME_ENABLED" default:"true"`
	HomeEventsSubName     string `envconfig:"SUBSCRIPTION_HOME_NAME"`
	AccountLinkingEnabled bool   `envconfig:"SUBSCRIPTION_ACCOUNT_LINKING_ENABLED"`
	AccountLinkingSubName string `envconfig:"SUBSCRIPTION_ACCOUNT_LINKING_NAME"`
}

type AWS struct {
	DefaultRegion             string `envconfig:"AWS_REGION" default:"us-east-1"`
	AccessKeyID               string `envconfig:"AWS_ACCESS_KEY_ID"`
	SecretAccessKey           string `envconfig:"AWS_SECRET_ACCESS_KEY"`
	DynamoDBGoogleTableName   string `envconfig:"AWS_DYNAMODB_GOOGLE_TABLE_NAME"`
	DynamoDBGoogleTableIndex  string `envconfig:"AWS_DYNAMODB_GOOGLE_TABLE_INDEX"`
	DynamoDBGoogleTableRegion string `envconfig:"AWS_DYNAMODB_GOOGLE_TABLE_REGION" default:"us-east-2"`
}

type GCP struct {
	ProjectID string `envconfig:"GOOGLE_PROJECT_ID"`
}

type Prometheus struct {
	Enabled bool   `envconfig:"PROMETHEUS_ENABLED" default:"false"`
	Port    string `envconfig:"PROMETHEUS_PORT" default:"9999"`
}

// Config holds start up config information
type Config struct {
	PubSub
	Subscriptions
	AWS
	GCP
	Prometheus
	Env                        string `envconfig:"ENV"`
	Port                       string `envconfig:"PORT"`
	LogDebug                   bool   `envconfig:"LOG_DEBUG" default:"false"`
	HealthServerPort           string `envconfig:"HEALTH_SERVER_PORT" default:"8080"`
	GoogleConsumerEnabled      bool   `envconfig:"GOOGLE_CONSUMER_ENABLED" default:"false"`
	GoogleServiceAccountKey    string `envconfig:"GOOGLE_SERVICE_ACCOUNT_KEY"`
	AccountLinkingEventSources string `envconfig:"ACCOUNT_LINKING_EVENT_SOURCES"`
	RedisAddress               string `envconfig:"REDIS_ADDRESS"`
}

// LoadConfigs loads from environment variables.
// If there are any errors during load it will Panic
func LoadConfigs() Config {
	var cfg Config
	envconfig.MustProcess("", &cfg)
	return cfg
}
