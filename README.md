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

## Deployment instructions

1. Pull the image from GCP 

    ```bash
    docker pull us-docker.pkg.dev/hdx-art/t/hdx-collector:0.1.0
    ```

1. Create a config file

    Copy the hydrolix-config.yaml from this repo and update it with your url, 
    tables and transforms.

1. Add service account credentials 

    Create a [service account](https://docs.hydrolix.io/latest/self-managed/advanced-configuration/authentication-and-authorization/account-types/service-accounts-howto/)
    in your Hydrolix cluster and add the credentials to docker.

1. Adding credentials to docker

    1. Run the image manually
    
        Export the environment variables
        ``` 
          export HYDROLIX_PASSWORD=password here
          export HYDROLIX_USERNAME=cluster url here
        ```

        ```bash 
        docker run --name otel-collector -p 4317:4317 \
          -p 4318:4318 \
          -e HYDROLIX_PASSWORD \
          -e HYDROLIX_USERNAME \
          -v [Absolute path to your otel config]:/etc/otelcol/config.yaml \
          us-docker.pkg.dev/hdx-art/t/hdx-collector:0.1.0
        ```

    1. Docker compose
       
        Docker compose can read environment variables from the system 
        environment variables. 
        ``` 
       # docker-compose.yml
        version: '3.8'
        services:
        otel-collector:
        image: otelcol-hydrolix:latest
        container_name: otel-collector
        ports:
        - "4317:4317"
        - "4318:4318"
        environment:
        - HYDROLIX_USERNAME=${HYDROLIX_USERNAME}
        - HYDROLIX_PASSWORD=${HYDROLIX_PASSWORD}
        # or use env_file:
        env_file:
        - .env
        volumes:
        - ./local.yaml:/etc/otelcol/config.yaml
        ```
    1. [Kubernetes deployment](KUBERNETES_DEPLOYMENT.md)
