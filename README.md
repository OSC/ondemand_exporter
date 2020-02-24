# Open OnDemand Prometheus exporter

[![Build Status](https://circleci.com/gh/OSC/ondemand_exporter/tree/master.svg?style=shield)](https://circleci.com/gh/OSC/ondemand_exporter)
[![GitHub release](https://img.shields.io/github/v/release/OSC/ondemand_exporter?include_prereleases&sort=semver)](https://github.com/OSC/ondemand_exporter/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/OSC/ondemand_exporter/total)
[![codecov](https://codecov.io/gh/OSC/ondemand_exporter/branch/master/graph/badge.svg)](https://codecov.io/gh/OSC/ondemand_exporter)

The OnDemand exporter collects metrics specific to Open OnDemand.

All metrics are accessible via the `/metrics` location.

## Metrics

* `ondemand_active_puns` - Number of active PUNs (from `nginx_stage nginx_list`)
* `ondemand_rack_apps` - Number of running Rack apps
* `ondemand_node_apps` - Number of running Node apps
* `ondemand_websocket_connections` - Web socket connections reported by Apache mod_status
* `ondemand_unique_websocket_clients` - Web socket connections report by Apache mod_status unique by client
* `ondemand_client_connections` - Number of client connections reported by Apache mod_status
* `ondemand_unique_client_connections` - Number of unique client connects reported by Apache mod_status
* `ondemand_pun_cpu_percent` - CPU percent of all PUNs
* `ondemand_pun_memory_bytes{type="rss|vms"}` - Memory RSS or virtual memory of all PUNs
* `ondemand_pun_memory_percent` - Percent memory used by all PUNs

Exporter metrics specific to status of the exporter

* `ondemand_exporter_collect_failures_total` - Counter of collection failures since exporter startup
* `ondemand_exporter_collector_duration_seconds{collector="apache|process|puns"}` - Duration of each collector
* `ondemand_exporter_error` - Status of the exporter, 0=no errors and 1=errors

## Flags

* `--listen` - Listen address, defaults to `:9301`
* `--apache-status` - The URL to reach Apache's mod_status `/server-status` URL. If undefined the value will be determined by reading `ood_portal.yml`.

## Setup

### sudo

Ensure the user running `ondemand_exporter` can execute `/opt/ood/nginx_stage/sbin/nginx_stage nginx_list`.
The following sudo config assumes `ondemand_exporter` is running as `ondemand_exporter`.

```
Defaults:ondemand_exporter !syslog
Defaults:ondemand_exporter !requiretty
ondemand_exporter ALL=(ALL) NOPASSWD:/opt/ood/nginx_stage/sbin/nginx_stage nginx_list
```

### Apache mod_status

Must also ensure Apache `mod_status` is loaded and configured.
The below example should have `SERVERNAME` replaced with OnDemand configured `servername` defined in `/etc/ood/config/ood_portal.yml`.

/opt/rh/httpd24/root/etc/httpd/conf.modules.d/status.conf:
```
LoadModule status_module modules/mod_status.so
<Location /server-status>
    SetHandler server-status
    Require ip 127.0.0.1 ::1
    Require host SERVERNAME
</Location>
ExtendedStatus On

<IfModule mod_proxy.c>
    # Show Proxy LoadBalancer status in mod_status
    ProxyStatus On
</IfModule>
```

If the `--apache-status` flag is not used the server name used to query mod_status is read from `/etc/ood/config/ood_portal.yml` so ensure the user running `ondemand_exporter` can read this file.

## Install

Add the user that will run `ondemand_exporter`

```
groupadd -r ondemand_exporter
useradd -r -d /var/lib/ondemand_exporter -s /sbin/nologin -M -g ondemand_exporter -M ondemand_exporter
```

If building from source, build `ondemand_exporter` and install.

```
make build
cp ondemand_exporter /usr/local/bin/ondemand_exporter
```

If using pre-compiled binaries, download the necessary asset and install the binari.

```
VERSION=0.1.0
curl -o /tmp/ondemand_exporter-${VERSION}.tar.gz https://github.com/OSC/ondemand_exporter/releases/download/v${VERSION}/ondemand_exporter-${VERSION}.linux-amd64.tar.gz
tar xf /tmp/ondemand-${VERSION}.tar.gz -C /tmp --strip-components=1
cp /tmp/ondemand_exporter /usr/local/bin/ondemand_exporter
```

Add sudo rule, see [sudo section](#sudo)

Enable Apache mod_status, see [Apache mod_status](#apache-mod_status)

Add systemd unit file and start service

```
cp systemd/ondemand_exporter.service /etc/systemd/system/ondemand_exporter.service
systemctl daemon-reload
systemctl start ondemand_exporter
```

## Build from source

To produce the `ondemand_exporter` binary:

```
make build
```

or

```
go get github.com/OSC/ondemand_exporter
```

## Install Grafana dashboard

See `graphs` directory for Grafana graphs

