# Stage 1: Build the custom collector
FROM --platform=$BUILDPLATFORM golang:1.25.3 AS builder

ARG TARGETARCH
ARG OCB_VERSION=0.141.0

RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') && \
  curl --proto '=https' --tlsv1.2 -fL -o /usr/local/bin/ocb \
  "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv${OCB_VERSION}/ocb_${OCB_VERSION}_linux_${ARCH}" && \
  chmod +x /usr/local/bin/ocb

WORKDIR /build

COPY builder-config.yml .
COPY go.mod go.sum ./exporter/
COPY *.go ./exporter/

RUN GOARCH=${TARGETARCH} ocb --config builder-config.yml

# Stage 2: Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /

COPY --from=builder /build/dist/otelcol-hydrolix /otelcol-hydrolix
COPY hydrolix-config.yaml /etc/otelcol/config.yaml

EXPOSE 4317 4318

ENTRYPOINT ["/otelcol-hydrolix"]
CMD ["--config", "/etc/otelcol/config.yaml"]