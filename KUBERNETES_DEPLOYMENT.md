# Deploying Hydrolix Exporter to Kubernetes

This guide explains how to build and deploy the custom OpenTelemetry Collector
with the Hydrolix exporter to Kubernetes.

## Prerequisites

- Docker installed and running
- `kubectl` configured to access your Kubernetes cluster
- Hydrolix credentials

## Step 1 (Option A): Pull the image from the GCP container registry

```bash
docker pull us-docker.pkg.dev/hdx-art/t/hdx-collector:0.1.0
```

## Step 1 (Option B): Build the Custom Collector Image

### Manual Build

```bash
cd path-to-repo/opentelemetry-collector-contrib

# Build the image
docker build -f Dockerfile.custom -t otel-collector-hydrolix:latest .
```

## Step 2: Create Hydrolix Credentials Secret

```bash
kubectl create secret generic hydrolix-credentials \
  --from-literal=endpoint="https://my-cluster.hydrolix.live/ingest/event" \
  --from-literal=hdx_bearer_token="your-bearer-token" \
  -n your-namespace
```

## Step 3: Update and Deploy the Kubernetes Configuration

1. Edit `k8s-deployment-example.yaml`:
   - Update the `image:` field with your image name
   - If using a private registry, uncomment `imagePullSecrets`
   - Update Hydrolix credentials in the Secret

2. Apply the configuration:

```bash
kubectl apply -f k8s-deployment-example.yaml
```

## Step 4: Verify the Deployment

```bash
# Check if the pod is running
kubectl get pods -n your-namespace

# View logs
kubectl logs -n your-namespace

# Check for successful metric exports (look for debug logs)
kubectl logs -n your-namespace deployment/otel-collector-hydrolix | grep -i hydrolix
```

Expected log output:
```
successfully sent metrics to Hydrolix (metric_count=15, table=my-org.metrics)
```

## Troubleshooting

### Exporter Errors

Check the logs for error messages:

```bash
kubectl logs -n your-namespace deployment/otel-collector-hydrolix | grep -i error
```

Common errors:
- **401/403**: Wrong Hydrolix credentials
- **404**: Wrong endpoint, table, or transform name
- **Connection refused**: Network/firewall issues

### Verifying the Exporter is Being Used

```bash
# The config should show "hydrolix" in exporters
kubectl get configmap otel-collector-config -n your-namespace -o yaml

# Check that the custom image is being used
kubectl get deployment otel-collector-hydrolix -n your-namespace -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Configuration Reference

### Hydrolix Exporter Config

### Multiple Exporters

You must use multiple exporter destinations inorder to specify the destination 
table and transform:

```yaml
exporters:
  hydrolix/metrics:
    endpoint: https://my-cluster.hydrolix.live/ingest/event
    hdx_table: my-org.metrics
    hdx_transform: tf_otel_metrics
    hdx_bearer_token: ${env:HYDROLIX_BEARER_TOKEN}
    # ...

  hydrolix/logs:
    endpoint: https://my-cluster.hydrolix.live/ingest/event
    hdx_table: my-org.logs
    hdx_transform: tf_my-org.logs
    hdx_bearer_token: ${env:HYDROLIX_BEARER_TOKEN}
    # ...

  hydrolix/traces:
    endpoint: https://your-hydrolix-cluster.hydrolix.io/ingest/event
    hdx_table: myorg.spans
    hdx_transform: tf_otel_spans
    hdx_bearer_token: ${env:HYDROLIX_BEARER_TOKEN}
    # ...

  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [kubeletstats]
      processors: [batch]
      exporters: [hydrolix/metrics, debug]
```

## Production Considerations

1. **Use versioned tags** instead of `latest`
   ```yaml
   image: us-docker.pkg.dev/hdx-art/t/hdx-collector:0.1.0
   imagePullPolicy: IfNotPresent
   ```

2. **Set resource limits** appropriately
   ```yaml
   resources:
     limits:
       memory: 256Mi
       cpu: 200m
     requests:
       memory: 128Mi
       cpu: 100m
   ```

3. **Enable retry and queuing** for reliability
   ```yaml
   exporters:
     hydrolix:
       # ... other config
       retry_on_failure:
         enabled: true
         initial_interval: 5s
         max_interval: 30s
         max_elapsed_time: 5m
       sending_queue:
         enabled: true
         num_consumers: 10
         queue_size: 1000
   ```

4. **Monitor the collector** itself
   ```yaml
   service:
     telemetry:
       logs:
         level: info
       metrics:
         level: detailed  # Enable collector metrics
         address: :8888
   ```

## Additional Resources

- [OpenTelemetry Collector Configuration](https://opentelemetry.io/docs/collector/configuration/)
- [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
