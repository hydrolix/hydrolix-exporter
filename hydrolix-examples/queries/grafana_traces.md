# Grafana Tracing view

This guide shows how to view traces stored in Hydrolix using Grafana's built-in Traces visualization.

## Overview

Grafana's Traces view provides a visual timeline representation of distributed traces, showing:
- Span hierarchies and dependencies
- Service relationships and call paths
- Timing and duration of each operation
- Span attributes and metadata
- Log events within spans

## Setup

## Create Query Variables

### Service name 

1. Go to **Dashboard Settings → Variables**
2. Click **Add variable**
3. Configure:
    - **Name**: `ServiceName`
    - **Type**: Query
    - **Query**:
      ```sql
      SELECT DISTINCT serviceName
      FROM YOUR_SPANS_TABLE
      WHERE $__timeFilter(timestamp)
      ORDER BY serviceName
      ```
    - **Multi-value**: Off
4. Save the variable

### Trace ID

1. Go to **Dashboard Settings → Variables**
1. Click **Add variable**
1. Configure:
    - **Name**: `traceid`
    - **Type**: Query
    - **Query**:
      ```sql
      SELECT 
        CONCAT(name, ' (', traceId, ')')
        FROM YOUR_SPANS_TABLE
        WHERE $__timeFilter(timestamp)
        AND (parentSpanId = '' OR parentSpanId IS NULL)
      ```
    - **Multi-value**: Off
1. Save the variable

## Using Traces View

### What You'll See

The Traces view displays:
- **Timeline**: Visual representation of all spans in the trace
- **Service Map**: Shows which services were involved
- **Span Details**: Click any span to see attributes, tags, and events
- **Duration**: Time spent in each operation
- **Logs**: Event logs attached to spans

## List All Traces

Before viewing individual traces, you'll typically want to see a list of available traces to identify which ones to investigate.

### Query to List Traces

This query shows root spans (the top-level operation in each trace) to give you an overview:

```sql
SELECT
  s.serviceName AS "Service Name",
  s.name AS "Trace name",
  dateDiff('ms', s.startTime, s.endTime) AS Duration,
  if(statusCode = 2, 'true', 'false') AS error,
  s.timestamp AS "Start time",
  s.traceId
FROM YOUR_SPANS_TABLE AS s
WHERE serviceName = '${ServiceName}'
  AND $__timeFilter(timestamp)
  AND (parentSpanId = '' OR parentSpanId IS NULL)
ORDER BY timestamp DESC
LIMIT 100
```

**What this query does:**
- Shows only root spans (traces without a parent) to avoid duplicate entries
- Filters by service name using a `${ServiceName}` variable
- Shows error status (statusCode = 2 means error in OpenTelemetry)
- Uses Grafana's `$__timeFilter` for time range filtering
- Limits to 100 most recent traces

### Creating a Dashboard with Trace Links

To make trace IDs clickable and open the detailed trace view:

#### Create the Dashboard Panel

1. Create a new dashboard or edit an existing one
2. Add a **Table** panel
3. Set your Hydrolix data source
4. Paste the query above (replace `YOUR_SPANS_TABLE` with your table name)

#### Add Data Link to Trace ID Column

Add a link to update the traces view panel that will be created in a later step.

1. In the Table panel, go to **Data links and actions**
1. Click **Add link**
1. **Title**: `View Trace`
1. Configure link:

    ```
    /d/YOUR_DASHBOARD_UID/traces?orgId=YOUR_ORG_ID&from=${__from}&to=${__to}&var-traceid=${__data.fields.traceId}
    ```

    Replace:
    - `YOUR_DASHBOARD_UID` - Your traces dashboard UID (found in dashboard URL)
    - `traces` - Your dashboard's URL slug
    - `var-traceid` - The variable name in your traces dashboard that filters by trace ID

    **Finding your dashboard UID:**
    1. View your current dashboard
    1. Copy the UID after `/d/`

### Result

Now when you view your dashboard:
1. Select a service from the **ServiceName** dropdown
2. See a table of recent traces for that service
3. Click any **traceId** to open the full trace visualization
4. The trace view will show all spans with timing, attributes, and logs

## View Individual Trace Details

Once you've identified a trace to investigate (either from the list below or from logs), use this query to view the full trace:

## Grafana Traces in Explore

[Grafana documentation](https://grafana.com/docs/grafana/latest/visualizations/explore/trace-integration/)

```
SELECT
  traceId AS traceID,
  spanId AS spanID,
  name AS operationName,
  parentSpanId AS parentSpanID,
  serviceName AS serviceName,
  statusCode,
  dateDiff('ms', startTime, endTime) AS duration,
  startTime,
  arrayMap(
    kv -> map('key', kv.1, 'value', kv.2),
    CAST(tags AS Array(Tuple(String, Nullable(String))))
  ) AS tags,
  arrayMap(
    kv -> map('key', kv.1, 'value', kv.2),
    CAST(serviceTags AS Array(Tuple(String, Nullable(String))))
  ) AS serviceTags,
  arrayMap(
    log_item -> tuple(
      log_item['name'],
      log_item['timestamp'],
      arrayMap(
        kv -> map('key', kv.1, 'value', kv.2),
        JSONExtractKeysAndValues(assumeNotNull(log_item['field']), 'String')
      )
    )::Tuple(
      name String,
      timestamp String,
      fields Array(Map(String, String))
    ),
    logs
  ) AS logs
  FROM YOUR_SPANS_TABLE
  WHERE traceId = extractAll('${traceid}', '\\((.+)\\)')[1]
    AND timestamp >= $__fromTime AND timestamp <= $__toTime
ORDER BY timestamp DESC
```

## Query Explanation

This query transforms Hydrolix span data into the format required by Grafana's Traces view:

### Key Transformations

1. **Field Mapping**: Maps Hydrolix span fields to Grafana's expected column names
   - `traceId` → `traceID`
   - `spanId` → `spanID`
   - `name` → `operationName`
   - `parentSpanId` → `parentSpanID`

2. **Duration Calculation**: Computes span duration in milliseconds from start and end times
   ```sql
   dateDiff('ms', startTime, endTime) AS duration
   ```

3. **Attribute Conversion**: Converts flat map attributes to Grafana's key-value array format
   - **tags**: Span-level attributes (HTTP method, status codes, etc.)
   - **serviceTags**: Resource-level attributes (service name, version, etc.)

   Both use the pattern:
   ```sql
   arrayMap(
     kv -> map('key', kv.1, 'value', kv.2),
     CAST(tags AS Array(Tuple(String, Nullable(String))))
   )
   ```

4. **Span Logs**: Processes span events/logs with their attributes
   - Extracts event name and timestamp
   - Converts event attributes from JSON object to key-value array
   - Uses `JSONExtractKeysAndValues` to parse nested field objects

### Variables

- **${traceid}**: The trace ID to query (required)
- **$__fromTime** / **$__toTime**: Grafana time range variables (automatic)
- **YOUR_SPANS_TABLE**: Replace with your actual Hydrolix spans table name

### Important Notes

- The query converts attributes from the new flat map format (`{"key": "value"}`) to Grafana's expected array format (`[{"key": "key", "value": "value"}]`)
- This maintains compatibility with Grafana's Traces UI expectations
- Time filtering is important for query performance on large trace datasets