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
* `ondemand_pun_cpu_time` - CPU time of all PUNs in seconds
* `ondemand_pun_memory_bytes{type="rss|vms"}` - Memory RSS or virtual memory of all PUNs
* `ondemand_pun_memory_percent` - Percent memory used by all PUNs
* `ondemand_passenger_instances` - Number of Passenger instances
* `ondemand_passenger_app_count` - Count of passenger instances of an app
* `ondemand_passenger_app_processes` - Process count of an app
* `ondemand_passenger_app_rss_bytes` - RSS of passenger apps
* `ondemand_passenger_real_memory_bytes` - Real Memory of passenger apps [ref](https://www.phusionpassenger.com/library/indepth/accurately_measuring_memory_usage.html)
* `ondemand_passenger_app_cpu_percent` - CPU percent of passenger apps
* `ondemand_passenger_app_requests_total` - Requests made to passenger apps
* `ondemand_passenger_app_average_runtime_seconds` - Average runtime in seconds of passenger apps

Exporter metrics specific to status of the exporter

* `ondemand_exporter_collect_duration_seconds{collector="apache|process|puns"}` - Duration of each collector
* `ondemand_exporter_collect_timeout{collector="apache|process|puns"}` - Indicates a collector timed out
* `ondemand_exporter_collect_error{collector="apache|process|puns"}` - Indicates error with a collector, 0=no errors and 1=errors

## Flags

* `--web.listen-address` - Listen address, defaults to `:9301`
* `--collector.apache.status-url` - The URL to reach Apache's mod_status `/server-status` URL. If undefined the value will be determined by reading `ood_portal.yml`.

## Setup

### sudo

Ensure the user running `ondemand_exporter` can execute `/opt/ood/nginx_stage/sbin/nginx_stage nginx_list` and `/usr/sbin/ondemand-passenger-status`.

An example of this file exists in [files/sudo](files/sudo), and this file could be copied using something like the following:

```
install -m 0440 -o root -g root files/sudo /etc/sudoers.d/ondemand_exporter
```

### Apache mod_status

Must also ensure Apache `mod_status` is loaded and configured.
A complete example of using a dedicated mod_status port for `localhost` is provided in [files/apache.conf](files/apache.conf)

This file would be installed to `/opt/rh/httpd24/root/etc/httpd/conf.d/ondemand_exporter.conf` for systems using SCL.

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
cp ondemand_exporter /usr/bin/ondemand_exporter
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
cp files/ondemand_exporter.service /etc/systemd/system/ondemand_exporter.service
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

