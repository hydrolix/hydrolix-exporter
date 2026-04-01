package hydrolixexporter

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsExporter struct {
	config   *Config
	settings exporter.Settings
	client   *http.Client
	logger   *zap.Logger
}

type HydrolixLog struct {
	Timestamp             uint64                 `json:"timestamp"`
	ObservedTimestamp     uint64                 `json:"observed_timestamp,omitempty"`
	TraceID               string                 `json:"traceId,omitempty"`
	SpanID                string                 `json:"spanId,omitempty"`
	TraceFlags            uint32                 `json:"trace_flags,omitempty"`
	SeverityText          string                 `json:"severity_text,omitempty"`
	SeverityNumber        int32                  `json:"severity_number,omitempty"`
	Body                  string                 `json:"body,omitempty"`
	LogAttributes         map[string]interface{} `json:"tags"`
	ResourceAttributes    map[string]interface{} `json:"serviceTags"`
	ResourceSchemaUrl     string                 `json:"resource_schema_url,omitempty"`
	ScopeName             string                 `json:"scope_name,omitempty"`
	ScopeVersion          string                 `json:"scope_version,omitempty"`
	ScopeAttributes       map[string]interface{} `json:"scope_attributes,omitempty"`
	ScopeDroppedAttrCount uint32                 `json:"scope_dropped_attr_count,omitempty"`
	ScopeSchemaUrl        string                 `json:"scope_schema_url,omitempty"`
	Flags                 uint32                 `json:"flags,omitempty"`
	ServiceName           string                 `json:"serviceName,omitempty"`
	HTTPStatusCode        string                 `json:"httpStatusCode,omitempty"`
	HTTPRoute             string                 `json:"httpRoute,omitempty"`
	HTTPMethod            string                 `json:"httpMethod,omitempty"`
}

func newLogsExporter(config *Config, set exporter.Settings) *logsExporter {
	return &logsExporter{
		config:   config,
		settings: set,
		logger:   set.Logger,
	}
}

func (e *logsExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.config.ClientConfig.ToClient(ctx, host.GetExtensions(), e.settings.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client
	return nil
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	logs := e.convertToHydrolixLogs(ld)
	sent, err := sendBatches(ctx, logs, e.config, e.client, e.logger, "logs")
	if err != nil {
		return consumererror.NewLogs(err, subLogs(ld, sent))
	}
	return nil
}

func (e *logsExporter) convertToHydrolixLogs(ld plog.Logs) []HydrolixLog {
	var logs []HydrolixLog

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resource := rl.Resource()
		resourceAttrs := convertAttributes(resource.Attributes())
		resourceSchemaUrl := rl.SchemaUrl()

		serviceName := extractStringAttr(resource.Attributes(), "service.name")

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scope := sl.Scope()
			scopeAttrs := convertAttributes(scope.Attributes())

			for k := 0; k < sl.LogRecords().Len(); k++ {
				logRecord := sl.LogRecords().At(k)

				hdxLog := HydrolixLog{
					Timestamp:             uint64(logRecord.Timestamp()),
					ObservedTimestamp:     uint64(logRecord.ObservedTimestamp()),
					TraceID:               logRecord.TraceID().String(),
					SpanID:                logRecord.SpanID().String(),
					TraceFlags:            uint32(logRecord.Flags()),
					SeverityText:          logRecord.SeverityText(),
					SeverityNumber:        int32(logRecord.SeverityNumber()),
					Body:                  logRecord.Body().AsString(),
					LogAttributes:         convertAttributes(logRecord.Attributes()),
					ResourceAttributes:    resourceAttrs,
					ResourceSchemaUrl:     resourceSchemaUrl,
					ScopeName:             scope.Name(),
					ScopeVersion:          scope.Version(),
					ScopeAttributes:       scopeAttrs,
					ScopeDroppedAttrCount: scope.DroppedAttributesCount(),
					ScopeSchemaUrl:        sl.SchemaUrl(),
					Flags:                 uint32(logRecord.Flags()),
					ServiceName:           serviceName,
					HTTPStatusCode:        extractStringAttr(logRecord.Attributes(), "http.response.status_code"),
					HTTPRoute:             extractStringAttr(logRecord.Attributes(), "http.route"),
					HTTPMethod:            extractStringAttr(logRecord.Attributes(), "http.request.method"),
				}

				logs = append(logs, hdxLog)
			}
		}
	}

	return logs
}
