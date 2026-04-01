// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestConvertAttributes(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("string_key", "string_value")
	attrs.PutInt("int_key", 42)
	attrs.PutDouble("double_key", 3.14)
	attrs.PutBool("bool_key", true)

	result := convertAttributes(attrs)

	assert.Len(t, result, 4)

	assert.Equal(t, "string_value", result["string_key"])
	assert.Equal(t, "42", result["int_key"])
	assert.Equal(t, "3.140000", result["double_key"])
	assert.Equal(t, "true", result["bool_key"])
}

func TestConvertAttributesEmpty(t *testing.T) {
	attrs := pcommon.NewMap()
	result := convertAttributes(attrs)
	assert.Empty(t, result)
}

func TestAttributeValueToInterface(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() pcommon.Value
		expected string
	}{
		{
			name: "string value",
			setup: func() pcommon.Value {
				v := pcommon.NewValueStr("test")
				return v
			},
			expected: "test",
		},
		{
			name: "int value",
			setup: func() pcommon.Value {
				v := pcommon.NewValueInt(123)
				return v
			},
			expected: "123",
		},
		{
			name: "double value",
			setup: func() pcommon.Value {
				v := pcommon.NewValueDouble(123.45)
				return v
			},
			expected: "123.450000",
		},
		{
			name: "bool value true",
			setup: func() pcommon.Value {
				v := pcommon.NewValueBool(true)
				return v
			},
			expected: "true",
		},
		{
			name: "bool value false",
			setup: func() pcommon.Value {
				v := pcommon.NewValueBool(false)
				return v
			},
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.setup()
			result := attributeValueToInterface(value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAttributeValueToInterface_Bytes(t *testing.T) {
	value := pcommon.NewValueBytes()
	value.Bytes().FromRaw([]byte{1, 2, 3, 4})

	result := attributeValueToInterface(value)
	assert.Equal(t, []byte{1, 2, 3, 4}, result)
}

func TestExtractStringAttr(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("existing_key", "value")
	attrs.PutInt("int_key", 42)

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "existing string attribute",
			key:      "existing_key",
			expected: "value",
		},
		{
			name:     "non-existing attribute",
			key:      "non_existing_key",
			expected: "",
		},
		{
			name:     "int attribute converted to string",
			key:      "int_key",
			expected: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStringAttr(attrs, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractStringAttrEmpty(t *testing.T) {
	attrs := pcommon.NewMap()
	result := extractStringAttr(attrs, "any_key")
	assert.Equal(t, "", result)
}

func TestConvertAttributesWithSpecialCharacters(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("special.key", "special/value")
	attrs.PutStr("unicode_key", "测试")

	result := convertAttributes(attrs)

	assert.Len(t, result, 2)

	assert.Equal(t, "special/value", result["special.key"])
	assert.Equal(t, "测试", result["unicode_key"])
}

func TestConvertAttributesWithLargeValues(t *testing.T) {
	attrs := pcommon.NewMap()
	largeString := string(make([]byte, 10000))
	attrs.PutStr("large_key", largeString)

	result := convertAttributes(attrs)

	assert.Len(t, result, 1)
	assert.Contains(t, result, "large_key")
	assert.Equal(t, largeString, result["large_key"])
}

func TestSendBatches_SmallPayload(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		MaxPayloadSize: 1024 * 1024, // 1MB
		HDXTable:       "test_table",
		HDXTransform:   "test_transform",
		HDXUsername:    "user",
		HDXPassword:    "pass",
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	err := exporter.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	items := []HydrolixLog{
		{Body: "log1", ServiceName: "svc"},
		{Body: "log2", ServiceName: "svc"},
	}

	sent, err := sendBatches(context.Background(), items, cfg, exporter.client, exporter.logger, "logs")
	require.NoError(t, err)
	assert.Equal(t, 2, sent)
	assert.Equal(t, 1, requestCount, "small payload should be sent in a single request")
}

func TestSendBatches_LargePayloadSplits(t *testing.T) {
	var requestCount int
	var receivedItems [][]HydrolixLog
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var items []HydrolixLog
		require.NoError(t, json.Unmarshal(body, &items))
		receivedItems = append(receivedItems, items)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		MaxPayloadSize: 100, // very small to force splitting
		HDXTable:       "test_table",
		HDXTransform:   "test_transform",
		HDXUsername:    "user",
		HDXPassword:    "pass",
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	err := exporter.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	items := make([]HydrolixLog, 10)
	for i := range items {
		items[i] = HydrolixLog{Body: "log message body here", ServiceName: "test-service"}
	}

	sent, err := sendBatches(context.Background(), items, cfg, exporter.client, exporter.logger, "logs")
	require.NoError(t, err)
	assert.Equal(t, 10, sent)
	assert.Greater(t, requestCount, 1, "large payload should be split into multiple requests")

	// Verify all items were sent
	totalReceived := 0
	for _, batch := range receivedItems {
		totalReceived += len(batch)
	}
	assert.Equal(t, 10, totalReceived, "all items should be received across chunks")
}

func TestSendBatches_EmptySlice(t *testing.T) {
	cfg := &Config{
		MaxPayloadSize: 1024,
	}

	logger := zap.NewNop()
	sent, err := sendBatches(context.Background(), []HydrolixLog{}, cfg, nil, logger, "logs")
	require.NoError(t, err)
	assert.Equal(t, 0, sent)
}

func TestSendBatches_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		MaxPayloadSize: 1024 * 1024,
		HDXTable:       "test_table",
		HDXTransform:   "test_transform",
		HDXUsername:    "user",
		HDXPassword:    "pass",
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	err := exporter.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	items := []HydrolixLog{{Body: "test"}}
	sent, err := sendBatches(context.Background(), items, cfg, exporter.client, exporter.logger, "logs")
	require.Error(t, err)
	assert.Equal(t, 0, sent)
	assert.Contains(t, err.Error(), "unexpected status code")
}

func TestSendBatches_PartialFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		MaxPayloadSize: 100, // force splitting into multiple batches
		HDXTable:       "test_table",
		HDXTransform:   "test_transform",
		HDXUsername:    "user",
		HDXPassword:    "pass",
	}

	exporter := newLogsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	err := exporter.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	items := make([]HydrolixLog, 10)
	for i := range items {
		items[i] = HydrolixLog{Body: "log message body here", ServiceName: "test-service"}
	}

	sent, err := sendBatches(context.Background(), items, cfg, exporter.client, exporter.logger, "logs")
	require.Error(t, err)
	assert.Greater(t, sent, 0, "some items should have been sent before failure")
	assert.Less(t, sent, 10, "not all items should have been sent")
}

func TestSubLogs(t *testing.T) {
	ld := plog.NewLogs()

	// Resource 1 with 2 scope logs
	rl1 := ld.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", "svc1")
	sl1 := rl1.ScopeLogs().AppendEmpty()
	sl1.Scope().SetName("scope1")
	sl1.LogRecords().AppendEmpty().Body().SetStr("log1")
	sl1.LogRecords().AppendEmpty().Body().SetStr("log2")
	sl2 := rl1.ScopeLogs().AppendEmpty()
	sl2.Scope().SetName("scope2")
	sl2.LogRecords().AppendEmpty().Body().SetStr("log3")

	// Resource 2
	rl2 := ld.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("service.name", "svc2")
	sl3 := rl2.ScopeLogs().AppendEmpty()
	sl3.LogRecords().AppendEmpty().Body().SetStr("log4")
	sl3.LogRecords().AppendEmpty().Body().SetStr("log5")

	// Skip first 3 records (log1, log2, log3) — should get log4, log5
	result := subLogs(ld, 3)
	assert.Equal(t, 1, result.ResourceLogs().Len())
	assert.Equal(t, "svc2", extractStringAttr(result.ResourceLogs().At(0).Resource().Attributes(), "service.name"))
	assert.Equal(t, 2, result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	assert.Equal(t, "log4", result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, "log5", result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Body().AsString())

	// Skip 1 — should get log2, log3, log4, log5
	result = subLogs(ld, 1)
	total := 0
	for i := 0; i < result.ResourceLogs().Len(); i++ {
		for j := 0; j < result.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			total += result.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
		}
	}
	assert.Equal(t, 4, total)

	// Skip 0 — should get everything
	result = subLogs(ld, 0)
	total = 0
	for i := 0; i < result.ResourceLogs().Len(); i++ {
		for j := 0; j < result.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			total += result.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
		}
	}
	assert.Equal(t, 5, total)

	// Skip all — should get empty
	result = subLogs(ld, 5)
	assert.Equal(t, 0, result.ResourceLogs().Len())
}

func TestSubTraces(t *testing.T) {
	td := ptrace.NewTraces()

	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc1")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty().SetName("span1")
	ss.Spans().AppendEmpty().SetName("span2")
	ss.Spans().AppendEmpty().SetName("span3")

	// Skip 2 — should get span3
	result := subTraces(td, 2)
	assert.Equal(t, 1, result.ResourceSpans().Len())
	assert.Equal(t, 1, result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
	assert.Equal(t, "span3", result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
}

func TestSubMetrics(t *testing.T) {
	md := pmetric.NewMetrics()

	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc1")
	sm := rm.ScopeMetrics().AppendEmpty()

	// Gauge with 2 data points
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("gauge1")
	g := m1.SetEmptyGauge()
	g.DataPoints().AppendEmpty().SetDoubleValue(1.0)
	g.DataPoints().AppendEmpty().SetDoubleValue(2.0)

	// Sum with 1 data point
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("sum1")
	s := m2.SetEmptySum()
	s.DataPoints().AppendEmpty().SetDoubleValue(3.0)

	// Skip 2 (both gauge DPs) — should get sum1 with 1 DP
	result := subMetrics(md, 2)
	assert.Equal(t, 1, result.ResourceMetrics().Len())
	assert.Equal(t, 1, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	assert.Equal(t, "sum1", result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())

	// Skip 1 (first gauge DP) — should get 1 gauge DP + sum1
	result = subMetrics(md, 1)
	metrics := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 2, metrics.Len())
	assert.Equal(t, "gauge1", metrics.At(0).Name())
	assert.Equal(t, 1, metrics.At(0).Gauge().DataPoints().Len())
	assert.Equal(t, 2.0, metrics.At(0).Gauge().DataPoints().At(0).DoubleValue())
	assert.Equal(t, "sum1", metrics.At(1).Name())
}
