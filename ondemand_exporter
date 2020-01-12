#!/usr/bin/env python

from __future__ import division
import argparse
import logging
import logging.handlers
from lxml import etree
from prometheus_client import start_http_server
from prometheus_client.core import GaugeMetricFamily, HistogramMetricFamily, CounterMetricFamily, REGISTRY
import psutil
import requests
import socket
import subprocess
import sys
import os
import time
import yaml

log = None

class OnDemandExporter(object):

    def __init__(self):
        self.active_puns = 0
        self.rack_apps = 0
        self.node_apps = 0
        self.max_pun_cpu_time_user = 0
        self.avg_pun_cpu_time_user = 0
        self.pun_cpu_time_user = 0
        self.max_pun_cpu_time_system = 0
        self.avg_pun_cpu_time_system = 0
        self.pun_cpu_time_system = 0
        self.max_pun_cpu_percent = 0
        self.avg_pun_cpu_percent = 0
        self.pun_cpu_percent = 0
        self.max_pun_memory_rss = 0
        self.avg_pun_memory_rss = 0
        self.pun_memory_rss = 0
        self.max_pun_memory_vms = 0
        self.avg_pun_memory_vms = 0
        self.pun_memory_vms = 0
        self.max_pun_memory_percent = 0
        self.avg_pun_memory_percent = 0
        self.pun_memory_percent = 0
        self.websocket_connections = 0
        self.unique_websocket_clients = 0
        self.client_connections = 0
        self.unique_client_connections = 0
        self.fqdn = socket.getfqdn()

    def collect(self):
        yield GaugeMetricFamily('ood_active_puns', 'Active PUNs', value=self.active_puns)
        yield GaugeMetricFamily('ood_rack_apps', 'Number of Rack Apps', value=self.rack_apps)
        yield GaugeMetricFamily('ood_node_apps', 'Number of NodeJS Apps', value=self.node_apps)
        max_pun_cpu_time = GaugeMetricFamily('ood_max_pun_cpu_time', 'Max CPU time of a PUN', labels=['mode'])
        max_pun_cpu_time.add_metric(['user'], self.max_pun_cpu_time_user)
        max_pun_cpu_time.add_metric(['system'], self.max_pun_cpu_time_system)
        yield max_pun_cpu_time
        avg_pun_cpu_time = GaugeMetricFamily('ood_avg_pun_cpu_time', 'Average CPU time of a PUN', labels=['mode'])
        avg_pun_cpu_time.add_metric(['user'], self.avg_pun_cpu_time_user)
        avg_pun_cpu_time.add_metric(['system'], self.avg_pun_cpu_time_system)
        yield avg_pun_cpu_time
        pun_cpu_time = CounterMetricFamily('ood_pun_cpu_time', 'CPU time of PUNs', labels=['mode'])
        pun_cpu_time.add_metric(['user'], self.pun_cpu_time_user)
        pun_cpu_time.add_metric(['system'], self.pun_cpu_time_system)
        yield pun_cpu_time
        yield GaugeMetricFamily('ood_max_pun_cpu_percent', 'Max CPU percent used by a PUN', value=self.max_pun_cpu_percent)
        yield GaugeMetricFamily('ood_avg_pun_cpu_percent', 'Average CPU percent used by a PUN', value=self.avg_pun_cpu_percent)
        yield GaugeMetricFamily('ood_pun_cpu_percent', 'CPU percent used by all PUNs', value=self.pun_cpu_percent)
        max_pun_memory = GaugeMetricFamily('ood_max_pun_memory', 'Max Memory used by PUN', labels=['type'])
        max_pun_memory.add_metric(['rss'], self.max_pun_memory_rss)
        max_pun_memory.add_metric(['vms'], self.max_pun_memory_vms)
        yield max_pun_memory
        avg_pun_memory = GaugeMetricFamily('ood_avg_pun_memory', 'Average Memory used by PUN', labels=['type'])
        avg_pun_memory.add_metric(['rss'], self.avg_pun_memory_rss)
        avg_pun_memory.add_metric(['vms'], self.avg_pun_memory_vms)
        yield avg_pun_memory
        pun_memory = GaugeMetricFamily('ood_pun_memory', 'Memory used by all PUNs', labels=['type'])
        pun_memory.add_metric(['rss'], self.pun_memory_rss)
        pun_memory.add_metric(['vms'], self.pun_memory_vms)
        yield pun_memory
        yield GaugeMetricFamily('ood_max_pun_memory_percent', 'Max Memory percent used by PUN', value=self.max_pun_memory_percent)
        yield GaugeMetricFamily('ood_avg_pun_memory_percent', 'Average Memory percent used by PUN', value=self.avg_pun_memory_percent)
        yield GaugeMetricFamily('ood_pun_memory_percent', 'Memory percent used by all PUNs', value=self.pun_memory_percent)
        yield GaugeMetricFamily('ood_websocket_connections', 'Number of Websocket Connections', value=self.websocket_connections)
        yield GaugeMetricFamily('ood_unique_websocket_clients', 'Number of unique Websocket Clients', value=self.unique_websocket_clients)
        yield GaugeMetricFamily('ood_client_connections', 'Number of client connections', value=self.client_connections)
        yield GaugeMetricFamily('ood_unique_client_connections', 'Number of unique client connections', value=self.unique_client_connections)

    def servername(self):
        ood_portal = {}
        with open('/etc/ood/config/ood_portal.yml', 'r') as f:
            ood_portal = yaml.load(f)
        servername = ood_portal.get('servername', self.fqdn)
        port = ood_portal.get('port', '80')
        _servername = "%s:%s" % (servername, port)
        return _servername

    def update_metrics(self):
        log.debug("UPDATING")
        active_puns = self.get_nginx_stage_metrics()
        self.get_process_metrics(active_puns)
        self.get_apache_status_metrics()

    def get_nginx_stage_metrics(self):
        cmd = ['sudo', '/opt/ood/nginx_stage/sbin/nginx_stage', 'nginx_list']
        log.debug("Executing: %s" % ' '.join(cmd))
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        out, err = proc.communicate()
        exit_code = proc.returncode
        if exit_code != 0:
            log.error('Exit code %s != 0' % exit_code)
            log.error('STDOUT: %s' % out)
            log.error('STDERR: %s' % err)
            return None
        log.debug('STDOUT: %s' % out)
        active_puns = []
        for line in out.splitlines():
            l = line.strip()
            active_puns.append(l)
        self.active_puns = len(active_puns)
        return active_puns

    def get_process_metrics(self, active_puns):
        pun_cpu_time_user =  [0.0]
        pun_cpu_time_system = [0.0]
        pun_cpu_percent = [0.0]
        pun_memory_rss = [0.0]
        pun_memory_vms = [0.0]
        pun_memory_percent = [0.0]
        rack_apps = 0
        node_apps = 0
        psutil_version = psutil.version_info
        if psutil_version[0] >= 2:
            attrs = ['name','cmdline','username','cpu_percent','cpu_times','memory_info','memory_percent']
        else:
            attrs = ['name','cmdline','username','get_cpu_percent','get_cpu_times','get_memory_info','get_memory_percent']
        for proc in psutil.process_iter():
            p = proc.as_dict(attrs=attrs)
            log.debug(p)
            if p['username'] not in active_puns:
                continue
            cmd = ' '.join(p['cmdline'])
            if 'rack-loader.rb' in cmd:
                rack_apps += 1
            if 'Passenger NodeApp' in cmd:
                node_apps += 1
            self.rack_apps = rack_apps
            self.node_apps = node_apps
            pun_cpu_time_user.append(p['cpu_times'].user)
            pun_cpu_time_system.append(p['cpu_times'].system)
            pun_cpu_percent.append(p['cpu_percent'])
            pun_memory_rss.append(p['memory_info'].rss)
            pun_memory_vms.append(p['memory_info'].vms)
            pun_memory_percent.append(p['memory_percent'])
        self.max_pun_cpu_time_user = max(pun_cpu_time_user)
        self.avg_pun_cpu_time_user = sum(pun_cpu_time_user)/len(pun_cpu_time_user)
        self.pun_cpu_time_user = sum(pun_cpu_time_user)
        self.max_pun_cpu_time_system = max(pun_cpu_time_user)
        self.avg_pun_cpu_time_system = sum(pun_cpu_time_user)/len(pun_cpu_time_user)
        self.pun_cpu_time_system = sum(pun_cpu_time_system)
        self.max_pun_cpu_percent = max(pun_cpu_percent)
        self.avg_pun_cpu_percent = sum(pun_cpu_percent)/len(pun_cpu_percent)
        self.pun_cpu_percent = sum(pun_cpu_percent)
        self.max_pun_memory_rss = max(pun_memory_rss)
        self.avg_pun_memory_rss = sum(pun_memory_rss)/len(pun_memory_rss)
        self.pun_memory_rss = sum(pun_memory_rss)
        self.max_pun_memory_vms = max(pun_memory_vms)
        self.avg_pun_memory_vms = sum(pun_memory_vms)/len(pun_memory_vms)
        self.pun_memory_vms = sum(pun_memory_vms)
        self.max_pun_memory_percent = max(pun_memory_percent)
        self.avg_pun_memory_percent = sum(pun_memory_percent)/len(pun_memory_percent)
        self.pun_memory_percent = sum(pun_memory_percent)

    def get_apache_status_metrics(self):
        servername = self.servername()
        if ':443' in servername:
            url = "https://%s/server-status" % servername
        else:
            url = "http://%s/server-status" % servername
        page = requests.get(url)
        tables = etree.HTML(page.content).xpath("//table")
        rows = iter(tables[0])
        headers = [col.text for col in next(rows)]
        log.debug("HEADERS: %s", headers)
        connections = []
        for row in rows:
            values = [col.text for col in row]
            log.debug("ROW: %s", values)
            connection = dict(zip(headers, values))
            connections.append(connection)
        log.debug(connections)
        websocket_connections = 0
        unique_websocket_clients = []
        client_connections = 0
        unique_client_connections = []
        for c in connections:
            request = c.get('Request', None)
            client = c.get('Client', None)
            if request is None or client is None:
                continue
            # Filter out connections not belonging to OOD
            if ('/node/' not in request and
                    '/rnode/' not in request and
                    '/pun/' not in request and
                    '/nginx/' not in request and
                    '/oidc' not in request and
                    '/discover' not in request and
                    '/register' not in request):
                log.debug("SKIP Request: %s", request)
                continue
            if '/node/' in request or '/rnode/' in request or 'websockify' in request:
                websocket_connections += 1
                if client not in unique_websocket_clients:
                    unique_websocket_clients.append(client)
            if client not in [self.fqdn, 'localhost', '127.0.0.1']:
                client_connections += 1
                if client not in unique_client_connections:
                    unique_client_connections.append(client)
        self.websocket_connections = websocket_connections
        self.client_connections = client_connections
        self.unique_websocket_clients = len(unique_websocket_clients)
        self.unique_client_connections = len(unique_client_connections)

def setup_logging(handlers, facility, level):
    global log

    log = logging.getLogger('ondemand_exporter')
    formatter = logging.Formatter('%(name)s: %(levelname)s: %(message)s')
    if handlers in ['syslog', 'both']:
        sh = logging.handlers.SysLogHandler(address='/dev/log', facility=facility)
        sh.setFormatter(formatter)
        log.addHandler(sh)
    if handlers in ['stdout', 'both']:
        ch = logging.StreamHandler()
        ch.setFormatter(formatter)
        log.addHandler(ch)
    lmap = {
        'CRITICAL': logging.CRITICAL,
        'ERROR': logging.ERROR,
        'WARNING': logging.WARNING,
        'INFO': logging.INFO,
        'DEBUG': logging.DEBUG,
        'NOTSET': logging.NOTSET
        }
    log.setLevel(lmap[level])

if __name__ == '__main__':
    log_level_choices = ['CRITICAL', 'ERROR', 'WARNING', 'INFO', 'DEBUG', 'NOTSET']
    log_choices = ['stdout', 'syslog', 'both']
    parser = argparse.ArgumentParser(formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('--log', action='store', default='stdout', choices=log_choices,
                      help='log to stdout and/or syslog.')
    parser.add_argument('--log-level', default='WARNING', choices=log_level_choices,
                      help='log to stdout and/or syslog')
    parser.add_argument('--log-facility', default='user',
                      help='facility to use when using syslog')
    parser.add_argument('--poll', type=int, default=60, help='how often to poll metrics')
    parser.add_argument('--port', type=int, default=9301, help='Port to listen on')
    args = parser.parse_args()
    setup_logging(args.log, args.log_facility, args.log_level)
    start_http_server(args.port)
    exporter = OnDemandExporter()
    REGISTRY.register(exporter)
    while True:
        exporter.update_metrics()
        time.sleep(args.poll)
