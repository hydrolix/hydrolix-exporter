# RPM Deployment Guide

This guide explains how to build and deploy the OpenTelemetry Collector with Hydrolix Exporter as an RPM package for RHEL/CentOS/Fedora systems.

## Prerequisites

### Build Environment
- RHEL/CentOS 7+ or Fedora
- Go 1.21 or later
- RPM build tools
- Git

### Install Build Dependencies

```bash
# RHEL/CentOS
sudo yum install -y rpm-build rpmdevtools golang git

# Fedora
sudo dnf install -y rpm-build rpmdevtools golang git
```

## Building the RPM

### 1. Set up RPM build environment

```bash
rpmdev-setuptree
```

This creates the following directory structure in your home directory:
```
~/rpmbuild/
├── BUILD/
├── RPMS/
├── SOURCES/
├── SPECS/
└── SRPMS/
```

### 2. Prepare the source

Clone the repository and create a tarball:

```bash
# Clone the repository
git clone https://github.com/hydrolix/hydrolix-exporter.git
cd hydrolix-exporter

# Create version variable
VERSION=1.0.1

# Create tarball
cd ..
tar czf ~/rpmbuild/SOURCES/otelcol-hydrolix-${VERSION}.tar.gz \
  --transform "s,^hydrolix-exporter,otelcol-hydrolix-${VERSION}," \
  hydrolix-exporter/
```

### 3. Copy spec file

```bash
cp hydrolix-exporter/packaging/rpm/otelcol-hydrolix.spec ~/rpmbuild/SPECS/
```

### 4. Build the RPM

```bash
# Build both source and binary RPMs
rpmbuild -ba ~/rpmbuild/SPECS/otelcol-hydrolix.spec

# Or build only binary RPM
rpmbuild -bb ~/rpmbuild/SPECS/otelcol-hydrolix.spec
```

The built RPM will be located at:
```
~/rpmbuild/RPMS/x86_64/otelcol-hydrolix-1.0.1-1.el8.x86_64.rpm
```

## Installation

### Install the RPM

```bash
sudo rpm -ivh ~/rpmbuild/RPMS/x86_64/otelcol-hydrolix-1.0.1-1.el8.x86_64.rpm
```

Or using yum/dnf:

```bash
sudo yum localinstall ~/rpmbuild/RPMS/x86_64/otelcol-hydrolix-1.0.1-1.el8.x86_64.rpm
```

### Post-Installation Configuration

The RPM installs the following files:
- Binary: `/usr/bin/otelcol-hydrolix`
- Config directory: `/etc/otelcol-hydrolix/`
- Example config: `/etc/otelcol-hydrolix/config.yaml.example`
- Systemd service: `/usr/lib/systemd/system/otelcol-hydrolix.service`
- Log directory: `/var/log/otelcol-hydrolix/`
- User: `otelcol` (created during installation)

### 1. Create Configuration File

```bash
# Copy and edit the example configuration
sudo cp /etc/otelcol-hydrolix/config.yaml.example /etc/otelcol-hydrolix/config.yaml
sudo vim /etc/otelcol-hydrolix/config.yaml
```

Example minimal configuration:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000

exporters:
  hydrolix/metrics:
    endpoint: https://your-cluster.hydrolix.live/ingest/event
    hdx_table: your_project.metrics_table
    hdx_transform: your_metrics_transform
    hdx_bearer_token: ${env:HYDROLIX_BEARER_TOKEN}
    timeout: 30s

  hydrolix/traces:
    endpoint: https://your-cluster.hydrolix.live/ingest/event
    hdx_table: your_project.traces_table
    hdx_transform: your_traces_transform
    hdx_bearer_token: ${env:HYDROLIX_BEARER_TOKEN}
    timeout: 30s

  hydrolix/logs:
    endpoint: https://your-cluster.hydrolix.live/ingest/event
    hdx_table: your_project.logs_table
    hdx_transform: your_logs_transform
    hdx_bearer_token: ${env:HYDROLIX_BEARER_TOKEN}
    timeout: 30s

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [hydrolix/traces]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [hydrolix/metrics]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [hydrolix/logs]
```

### 2. Set Environment Variables

Create the environment file with your credentials:

```bash
sudo vim /etc/otelcol-hydrolix/otelcol-hydrolix.conf
```

Add your authentication credentials:

```bash
# Using Bearer Token (recommended)
HYDROLIX_BEARER_TOKEN=your-token-here

# OR using Basic Authentication
# HYDROLIX_USERNAME=your-username
# HYDROLIX_PASSWORD=your-password
```

Set proper permissions:

```bash
sudo chown otelcol:otelcol /etc/otelcol-hydrolix/otelcol-hydrolix.conf
sudo chmod 600 /etc/otelcol-hydrolix/otelcol-hydrolix.conf
```

### 3. Set File Permissions

```bash
sudo chown -R otelcol:otelcol /etc/otelcol-hydrolix
sudo chmod 750 /etc/otelcol-hydrolix
sudo chmod 640 /etc/otelcol-hydrolix/config.yaml
```

### 4. Configure Firewall (if needed)

```bash
# Allow OTLP gRPC port
sudo firewall-cmd --permanent --add-port=4317/tcp

# Allow OTLP HTTP port
sudo firewall-cmd --permanent --add-port=4318/tcp

# Reload firewall
sudo firewall-cmd --reload
```

### 5. Start and Enable Service

```bash
# Start the service
sudo systemctl start otelcol-hydrolix

# Enable service to start on boot
sudo systemctl enable otelcol-hydrolix

# Check status
sudo systemctl status otelcol-hydrolix
```

## Service Management

### Check Service Status

```bash
sudo systemctl status otelcol-hydrolix
```

### View Logs

```bash
# View systemd logs
sudo journalctl -u otelcol-hydrolix -f

# View application logs
sudo tail -f /var/log/otelcol-hydrolix/otelcol.log
```

### Restart Service

```bash
sudo systemctl restart otelcol-hydrolix
```

### Stop Service

```bash
sudo systemctl stop otelcol-hydrolix
```

### Reload Configuration

After editing the config file:

```bash
sudo systemctl restart otelcol-hydrolix
```

## Testing the Installation

### Test with curl

```bash
# Test OTLP HTTP endpoint
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"stringValue": "test-service"}
        }]
      },
      "scopeSpans": []
    }]
  }'
```

### Check Connectivity to Hydrolix

```bash
# Test authentication
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://your-cluster.hydrolix.live/config/v1/projects
```

## Upgrading

To upgrade to a new version:

```bash
# Build the new RPM with updated version
rpmbuild -bb ~/rpmbuild/SPECS/otelcol-hydrolix.spec

# Upgrade the package
sudo rpm -Uvh ~/rpmbuild/RPMS/x86_64/otelcol-hydrolix-0.2.0-1.el8.x86_64.rpm

# Restart the service
sudo systemctl restart otelcol-hydrolix
```

## Uninstalling

To remove the package:

```bash
# Stop and disable the service
sudo systemctl stop otelcol-hydrolix
sudo systemctl disable otelcol-hydrolix

# Remove the RPM
sudo rpm -e otelcol-hydrolix

# Optionally remove user data
sudo rm -rf /var/log/otelcol-hydrolix
sudo rm -rf /etc/otelcol-hydrolix
```

Note: The `otelcol` user and group will remain after uninstallation. To remove them:

```bash
sudo userdel otelcol
sudo groupdel otelcol
```

## Troubleshooting

### Service won't start

1. Check systemd logs:
   ```bash
   sudo journalctl -u otelcol-hydrolix -n 50
   ```

2. Verify configuration:
   ```bash
   sudo -u otelcol /usr/bin/otelcol-hydrolix --config /etc/otelcol-hydrolix/config.yaml validate
   ```

3. Check file permissions:
   ```bash
   ls -la /etc/otelcol-hydrolix/
   ```

### Permission denied errors

Ensure the otelcol user has proper permissions:

```bash
sudo chown -R otelcol:otelcol /etc/otelcol-hydrolix
sudo chown -R otelcol:otelcol /var/log/otelcol-hydrolix
```

### Authentication failures (401 errors)

1. Verify environment variables are set correctly in `/etc/otelcol-hydrolix/otelcol-hydrolix.conf`
2. Ensure the file has correct permissions (600)
3. Test credentials manually with curl
4. Check for quotes or extra spaces in the bearer token

### Port already in use

Check if another service is using ports 4317 or 4318:

```bash
sudo ss -tlnp | grep -E ':(4317|4318)'
```

## Building for Multiple Architectures

To build for different architectures:

```bash
# For ARM64
rpmbuild --target aarch64 -bb ~/rpmbuild/SPECS/otelcol-hydrolix.spec

# For x86_64
rpmbuild --target x86_64 -bb ~/rpmbuild/SPECS/otelcol-hydrolix.spec
```
