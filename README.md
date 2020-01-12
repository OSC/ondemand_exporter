# Open OnDemand Prometheus exporter

## Metrics

* `ood_active_puns` - Number of active PUNs (from `nginx_stage nginx_list`)
* `ood_rack_apps` - Number of running Rack apps
* `ood_node_apps` - Number of running Node apps
* `ood_websocket_connections` - Web socket connections reported by Apache mod_status
* `ood_unique_websocket_clients` - Web socket connections report by Apache mod_status unique by client
* `ood_client_connections` - Number of client connections reported by Apache mod_status
* `ood_unique_client_connections` - Number of unique client connects reported by Apache mod_status
* `ood_max_pun_cpu_time{mode="user|system"}` - Max PUN CPU time
* `ood_avg_pun_cpu_time{mode="user|system"}` - Average PUN CPU time
* `ood_pun_cpu_time{mode="user|system"}` - Total PUN CPU time
* `ood_max_pun_cpu_percent` - Max PUN CPU percent
* `ood_avg_pun_cpu_percent` - Average PUN CPU percent
* `ood_pun_cpu_percent` - Total CPU percent of all PUNs
* `ood_max_pun_memory{type="rss|vms"}` - Max PUN RSS or virtual memory
* `ood_avg_pun_memory{type="rss|vms"}` - Average PUN RSS or virtual memory
* `ood_pun_memory{type="rss|vms"}` - Total PUN RSS or virtual memory
* `ood_max_pun_memory_percent` - Max PUN memory percent
* `ood_avg_pun_memory_percent` - Average PUN memory percent
* `ood_pun_memory_percent` - Total PUN memory percent

## Setup

Install dependencies (requires EPEL repo):

```
yum -y install python2-psutil
```

Install Prometheus Python client

```
pip install prometheus_client
```

Ensure the user running `ondemand_exporter.py` can execute `/opt/ood/nginx_stage/sbin/nginx_stage nginx_list`.  The following sudo config assumes `ondemand_exporter.py` is running as `nobody`.

```
Defaults:nobody !syslog
Defaults:nobody !requiretty
nobody ALL=(ALL) NOPASSWD:/opt/ood/nginx_stage/sbin/nginx_stage nginx_list
```

Must also ensure Apache `mod_status` is loaded and configured.  The below example should have `SERVERNAME` replaced with OnDemand configured `servername` defined in `/etc/ood/config/ood_portal.yml`.

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

The server name used to query mod_status is read from `/etc/ood/config/ood_portal.yml` so ensure the user running `ondemand_exporter.py` can read this file

## Install plugin

Install the necessary files and start the exporter service

```
cp ondemand_exporter.py /usr/local/bin/ondemand_exporter
#TODO Unit file
systemctl start ondemand_exporter
```

## Install Grafana dashboard

TODO

