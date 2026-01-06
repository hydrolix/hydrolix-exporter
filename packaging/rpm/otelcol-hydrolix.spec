Name:           otelcol-hydrolix
Version:        1.0.1
Release:        1%{?dist}
Summary:        OpenTelemetry Collector with Hydrolix Exporter

License:        Apache-2.0
URL:            https://github.com/hydrolix/hydrolix-exporter
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.21
BuildRequires:  systemd-rpm-macros
Requires:       systemd
Requires(pre):  shadow-utils

%description
OpenTelemetry Collector with custom Hydrolix exporter for metrics, traces, and logs.

%prep
%setup -q

%build
# Build the collector using ocb (OpenTelemetry Collector Builder)
go install go.opentelemetry.io/collector/cmd/builder@v0.112.0
$HOME/go/bin/builder --config=builder-config.yml

%install
# Create directories
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_sysconfdir}/otelcol-hydrolix
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_localstatedir}/log/otelcol-hydrolix

# Install binary
install -m 0755 otelcol-hydrolix/otelcol-hydrolix %{buildroot}%{_bindir}/otelcol-hydrolix

# Install systemd service
install -m 0644 packaging/rpm/otelcol-hydrolix.service %{buildroot}%{_unitdir}/otelcol-hydrolix.service

# Install example config
install -m 0644 hydrolix-config.yaml %{buildroot}%{_sysconfdir}/otelcol-hydrolix/config.yaml.example

# Install example environment file
install -m 0644 packaging/rpm/otelcol-hydrolix.conf %{buildroot}%{_sysconfdir}/otelcol-hydrolix/otelcol-hydrolix.conf.example

%pre
# Create user and group
getent group otelcol >/dev/null || groupadd -r otelcol
getent passwd otelcol >/dev/null || \
    useradd -r -g otelcol -d /var/lib/otelcol-hydrolix -s /sbin/nologin \
    -c "OpenTelemetry Collector" otelcol
exit 0

%post
%systemd_post otelcol-hydrolix.service

%preun
%systemd_preun otelcol-hydrolix.service

%postun
%systemd_postun_with_restart otelcol-hydrolix.service

%files
%{_bindir}/otelcol-hydrolix
%{_unitdir}/otelcol-hydrolix.service
%dir %attr(0755, otelcol, otelcol) %{_sysconfdir}/otelcol-hydrolix
%config(noreplace) %attr(0644, otelcol, otelcol) %{_sysconfdir}/otelcol-hydrolix/config.yaml.example
%config(noreplace) %attr(0600, otelcol, otelcol) %{_sysconfdir}/otelcol-hydrolix/otelcol-hydrolix.conf.example
%dir %attr(0755, otelcol, otelcol) %{_localstatedir}/log/otelcol-hydrolix

%changelog
* Sun Jan 05 2025 Your Name <your.email@example.com> - 1.0.1-1
- Added all OpenTelemetry Collector Contrib components
- Includes kubeletstats receiver, filter processor, and 100+ other components

* Sun Dec 22 2024 Your Name <your.email@example.com> - 1.0.0-1
- Initial RPM release
- Includes Hydrolix exporter for metrics, traces, and logs
