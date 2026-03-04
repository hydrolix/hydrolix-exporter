package hydrolixexporter

import (
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`

	RetryConfig configretry.BackOffConfig       `mapstructure:"retry_on_failure"`
	QueueConfig exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// Hydrolix-specific configuration
	HDXTable       string `mapstructure:"hdx_table"`
	HDXTransform   string `mapstructure:"hdx_transform"`
	HDXUsername    string `mapstructure:"hdx_username"`
	HDXPassword    string `mapstructure:"hdx_password"`
	HDXBearerToken string `mapstructure:"hdx_bearer_token"`
}
