# RPM Packaging Files

This directory contains the files needed to build an RPM package for the OpenTelemetry Collector with Hydrolix Exporter.

## Files

- **otelcol-hydrolix.spec** - RPM spec file defining the package build process
- **otelcol-hydrolix.service** - systemd service unit file for managing the collector
- **otelcol-hydrolix.conf** - Example environment file for configuration

## Usage

See the [RPM Deployment Guide](../../RPM_DEPLOYMENT.md) for complete instructions on building and deploying the RPM package.

## Quick Build

```bash
# Set up build environment
rpmdev-setuptree

# Create source tarball
VERSION=1.0.1
cd ../..
tar czf ~/rpmbuild/SOURCES/otelcol-hydrolix-${VERSION}.tar.gz \
  --transform "s,^hydrolix-exporter,otelcol-hydrolix-${VERSION}," \
  hydrolix-exporter/

# Copy spec file
cp hydrolix-exporter/packaging/rpm/otelcol-hydrolix.spec ~/rpmbuild/SPECS/

# Build RPM
rpmbuild -bb ~/rpmbuild/SPECS/otelcol-hydrolix.spec
```
