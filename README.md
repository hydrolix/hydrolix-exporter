# Hydrolix OTel exporter

| Metrics | Logs | Traces | 
| ------- | ---- | ------ |
| [![experimental](http://badges.github.io/stability-badges/dist/experimental.svg)](http://github.com/badges/stability-badges) | [![experimental](http://badges.github.io/stability-badges/dist/experimental.svg)](http://github.com/badges/stability-badges) | [![experimental](http://badges.github.io/stability-badges/dist/experimental.svg)](http://github.com/badges/stability-badges) |



## Setup 

### Required settings: 

- endpoint - The Hydrolix cluster ingest endpoint that you would like to send traces to
- hdx_table - The Hydrolix table name where telemetry data will be written
- hdx_transform - The Hydrolix transform name to apply to incoming data
- hdx_username - Username for authentication to the Hydrolix cluster
- hdx_password - Password for authentication to the Hydrolix cluster 

```
hydrolix/metrics:
    endpoint: https://your-hydrolix-cluster.hydrolix.io/ingest/event
    hdx_table: Your hdx table name here
    hdx_transform: your related transform
    hdx_username: ${env:HYDROLIX_USERNAME}
    hdx_password: ${env:HYDROLIX_PASSWORD}
```

## Deployment

- [Kubernetes deployment](KUBERNETES_DEPLOYMENT.md)