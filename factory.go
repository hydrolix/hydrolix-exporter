package hydrolixexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "hydrolix"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelBeta),
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelBeta),
		exporter.WithLogs(createLogsExporter, component.StabilityLevelBeta),
	)
}

func createDefaultConfig() component.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Compression = configcompression.TypeGzip
	clientConfig.Timeout = 30 * time.Second
	// We write large JSON payloads; tuning the write buffer reduces syscall overhead.
	// We read very little (small response body), so ReadBufferSize is left at default.
	clientConfig.WriteBufferSize = 512 * 1024
	return &Config{
		ClientConfig: clientConfig,
		RetryConfig:  configretry.NewDefaultBackOffConfig(),
		QueueConfig:  exporterhelper.NewDefaultQueueConfig(),
	}
}

func exporterOptions(config *Config) []exporterhelper.Option {
	return []exporterhelper.Option{
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: config.Timeout}),
		exporterhelper.WithRetry(config.RetryConfig),
		exporterhelper.WithQueue(config.QueueConfig),
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	config := cfg.(*Config)
	te := newTracesExporter(config, set)

	opts := append(exporterOptions(config), exporterhelper.WithStart(te.start))
	return exporterhelper.NewTraces(ctx, set, cfg, te.pushTraces, opts...)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	config := cfg.(*Config)
	me := newMetricsExporter(config, set)

	opts := append(exporterOptions(config), exporterhelper.WithStart(me.start))
	return exporterhelper.NewMetrics(ctx, set, cfg, me.pushMetrics, opts...)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	config := cfg.(*Config)
	le := newLogsExporter(config, set)

	opts := append(exporterOptions(config), exporterhelper.WithStart(le.start))
	return exporterhelper.NewLogs(ctx, set, cfg, le.pushLogs, opts...)
}
