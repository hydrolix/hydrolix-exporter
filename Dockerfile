# Stage 1: Build the custom collector
FROM golang:1.25.3 AS builder

# Install OpenTelemetry Collector Builder
ARG OCB_VERSION=0.141.0
RUN curl --proto '=https' --tlsv1.2 -fL -o /tmp/ocb \
    https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv${OCB_VERSION}/ocb_${OCB_VERSION}_linux_amd64 && \
    chmod +x /tmp/ocb && \
    mv /tmp/ocb /usr/local/bin/ocb

WORKDIR /build

# Copy the builder configuration and exporter source
COPY builder-config.yml .
COPY *.go ./
COPY go.mod go.sum ./
COPY internal/ ./internal/
COPY metadata.yaml ./

# Build the custom collector
RUN ocb --config builder-config.yml

# Stage 2: Create the runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy the built collector from the builder stage
COPY --from=builder /build/otelcol-hydrolix /otelcol-hydrolix

# Copy the default configuration
COPY hydrolix-config.yaml /etc/otelcol/config.yaml

# Expose the OTLP receiver ports
EXPOSE 4317 4318

# Set the entrypoint
ENTRYPOINT ["/otelcol-hydrolix"]
CMD ["--config", "/etc/otelcol/config.yaml"]