package hydrolixexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const defaultMaxPayloadSize = 5 * 1024 * 1024 // 5MB

// sendBatches serializes items individually, groups them into size-limited batches,
// and sends each batch as a JSON array. Returns the number of items successfully
// sent and any error encountered. Callers should use the sent count to construct
// a consumererror with only the unsent pdata portion.
func sendBatches[T any](ctx context.Context, items []T, config *Config, client *http.Client, logger *zap.Logger, signalType string) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	maxSize := config.MaxPayloadSize
	if maxSize <= 0 {
		maxSize = defaultMaxPayloadSize
	}

	// Serialize each item once to determine sizes.
	serialized := make([][]byte, len(items))
	for i, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal %s: %w", signalType, err)
		}
		serialized[i] = data
	}

	// Build and send size-limited batches.
	sent := 0
	batchStart := 0
	batchSize := 2 // JSON array brackets: [ ]

	for i := range serialized {
		separator := 0
		if i > batchStart {
			separator = 1 // comma between items
		}

		if batchSize+len(serialized[i])+separator > maxSize && i > batchStart {
			payload := buildJSONArray(serialized[batchStart:i])
			if err := sendPayload(ctx, payload, i-batchStart, config, client, logger, signalType); err != nil {
				return sent, err
			}
			sent += i - batchStart
			batchStart = i
			batchSize = 2
			separator = 0
		}
		batchSize += len(serialized[i]) + separator
	}

	// Flush remaining items.
	if batchStart < len(serialized) {
		payload := buildJSONArray(serialized[batchStart:])
		if err := sendPayload(ctx, payload, len(serialized)-batchStart, config, client, logger, signalType); err != nil {
			return sent, err
		}
		sent += len(serialized) - batchStart
	}

	return sent, nil
}

// buildJSONArray constructs a JSON array from pre-serialized items without
// re-marshaling, avoiding double serialization.
func buildJSONArray(items [][]byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(item)
	}
	buf.WriteByte(']')
	return buf.Bytes()
}

func sendPayload(ctx context.Context, payload []byte, count int, config *Config, client *http.Client, logger *zap.Logger, signalType string) error {
	if err := sendRequest(ctx, payload, config, client); err != nil {
		logger.Warn("failed to send "+signalType+" to Hydrolix",
			zap.Int("count", count),
			zap.Int("payload_bytes", len(payload)),
			zap.Error(err))
		return err
	}
	logger.Debug("successfully sent "+signalType+" to Hydrolix",
		zap.Int("count", count),
		zap.Int("payload_bytes", len(payload)),
		zap.String("table", config.HDXTable))
	return nil
}

func sendRequest(ctx context.Context, jsonData []byte, config *Config, client *http.Client) error {
	req, err := http.NewRequestWithContext(ctx, "POST", config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hdx-table", config.HDXTable)
	req.Header.Set("x-hdx-transform", config.HDXTransform)

	if config.HDXBearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.HDXBearerToken)
	} else {
		req.SetBasicAuth(config.HDXUsername, config.HDXPassword)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Hydrolix: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("unexpected status code: %d (failed to read response body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// subLogs returns a plog.Logs containing only the log records starting from
// the skip-th record in iteration order, preserving resource and scope context.
func subLogs(ld plog.Logs, skip int) plog.Logs {
	result := plog.NewLogs()
	remaining := skip

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		rlAdded := false

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			recordCount := sl.LogRecords().Len()

			if remaining >= recordCount {
				remaining -= recordCount
				continue
			}

			if !rlAdded {
				newRL := result.ResourceLogs().AppendEmpty()
				rl.Resource().CopyTo(newRL.Resource())
				newRL.SetSchemaUrl(rl.SchemaUrl())
				rlAdded = true
			}

			destRL := result.ResourceLogs().At(result.ResourceLogs().Len() - 1)
			newSL := destRL.ScopeLogs().AppendEmpty()
			sl.Scope().CopyTo(newSL.Scope())
			newSL.SetSchemaUrl(sl.SchemaUrl())

			startK := remaining
			remaining = 0
			for k := startK; k < recordCount; k++ {
				sl.LogRecords().At(k).CopyTo(newSL.LogRecords().AppendEmpty())
			}
		}
	}

	return result
}

// subTraces returns a ptrace.Traces containing only the spans starting from
// the skip-th span in iteration order, preserving resource and scope context.
func subTraces(td ptrace.Traces, skip int) ptrace.Traces {
	result := ptrace.NewTraces()
	remaining := skip

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		rsAdded := false

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			spanCount := ss.Spans().Len()

			if remaining >= spanCount {
				remaining -= spanCount
				continue
			}

			if !rsAdded {
				newRS := result.ResourceSpans().AppendEmpty()
				rs.Resource().CopyTo(newRS.Resource())
				newRS.SetSchemaUrl(rs.SchemaUrl())
				rsAdded = true
			}

			destRS := result.ResourceSpans().At(result.ResourceSpans().Len() - 1)
			newSS := destRS.ScopeSpans().AppendEmpty()
			ss.Scope().CopyTo(newSS.Scope())
			newSS.SetSchemaUrl(ss.SchemaUrl())

			startK := remaining
			remaining = 0
			for k := startK; k < spanCount; k++ {
				ss.Spans().At(k).CopyTo(newSS.Spans().AppendEmpty())
			}
		}
	}

	return result
}

// subMetrics returns a pmetric.Metrics containing only the data points starting
// from the skip-th data point in iteration order. Since each HydrolixMetric
// corresponds to one data point, skip counts data points across all metric types.
func subMetrics(md pmetric.Metrics, skip int) pmetric.Metrics {
	result := pmetric.NewMetrics()
	remaining := skip

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		rmAdded := false

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			smAdded := false

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				dpCount := metricDataPointCount(m)

				if remaining >= dpCount {
					remaining -= dpCount
					continue
				}

				if !rmAdded {
					newRM := result.ResourceMetrics().AppendEmpty()
					rm.Resource().CopyTo(newRM.Resource())
					newRM.SetSchemaUrl(rm.SchemaUrl())
					rmAdded = true
				}
				if !smAdded {
					destRM := result.ResourceMetrics().At(result.ResourceMetrics().Len() - 1)
					newSM := destRM.ScopeMetrics().AppendEmpty()
					sm.Scope().CopyTo(newSM.Scope())
					newSM.SetSchemaUrl(sm.SchemaUrl())
					smAdded = true
				}

				destSM := result.ResourceMetrics().At(result.ResourceMetrics().Len() - 1).
					ScopeMetrics().At(result.ResourceMetrics().At(result.ResourceMetrics().Len() - 1).ScopeMetrics().Len() - 1)
				newM := destSM.Metrics().AppendEmpty()
				m.CopyTo(newM)

				// If we need to skip some data points within this metric, remove them.
				if remaining > 0 {
					removeLeadingDataPoints(newM, remaining)
					remaining = 0
				}
			}
		}
	}

	return result
}

func metricDataPointCount(m pmetric.Metric) int {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return m.Gauge().DataPoints().Len()
	case pmetric.MetricTypeSum:
		return m.Sum().DataPoints().Len()
	case pmetric.MetricTypeHistogram:
		return m.Histogram().DataPoints().Len()
	case pmetric.MetricTypeExponentialHistogram:
		return m.ExponentialHistogram().DataPoints().Len()
	case pmetric.MetricTypeSummary:
		return m.Summary().DataPoints().Len()
	default:
		return 0
	}
}

func removeLeadingDataPoints(m pmetric.Metric, count int) {
	removed := 0
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		m.Gauge().DataPoints().RemoveIf(func(_ pmetric.NumberDataPoint) bool {
			if removed < count {
				removed++
				return true
			}
			return false
		})
	case pmetric.MetricTypeSum:
		m.Sum().DataPoints().RemoveIf(func(_ pmetric.NumberDataPoint) bool {
			if removed < count {
				removed++
				return true
			}
			return false
		})
	case pmetric.MetricTypeHistogram:
		m.Histogram().DataPoints().RemoveIf(func(_ pmetric.HistogramDataPoint) bool {
			if removed < count {
				removed++
				return true
			}
			return false
		})
	case pmetric.MetricTypeExponentialHistogram:
		m.ExponentialHistogram().DataPoints().RemoveIf(func(_ pmetric.ExponentialHistogramDataPoint) bool {
			if removed < count {
				removed++
				return true
			}
			return false
		})
	case pmetric.MetricTypeSummary:
		m.Summary().DataPoints().RemoveIf(func(_ pmetric.SummaryDataPoint) bool {
			if removed < count {
				removed++
				return true
			}
			return false
		})
	}
}

// convertAttributes converts OTLP attributes to a flat map
func convertAttributes(attrs pcommon.Map) map[string]interface{} {
	tags := make(map[string]interface{})

	attrs.Range(func(k string, v pcommon.Value) bool {
		tags[k] = attributeValueToInterface(v)
		return true
	})

	return tags
}

// attributeValueToInterface converts OTLP attribute value to interface{}
func attributeValueToInterface(v pcommon.Value) interface{} {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return fmt.Sprintf("%d", v.Int())
	case pcommon.ValueTypeDouble:
		return fmt.Sprintf("%f", v.Double())
	case pcommon.ValueTypeBool:
		return fmt.Sprintf("%t", v.Bool())
	case pcommon.ValueTypeBytes:
		return v.Bytes().AsRaw()
	default:
		return v.AsString()
	}
}

// extractStringAttr extracts a string attribute value by key
func extractStringAttr(attrs pcommon.Map, key string) string {
	if val, ok := attrs.Get(key); ok {
		return val.AsString()
	}
	return ""
}
