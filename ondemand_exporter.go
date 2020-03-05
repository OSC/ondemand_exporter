// MIT License
//
// Copyright (c) 2020 Ohio Supercomputer Center
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/OSC/ondemand_exporter/collectors"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

var (
	listenAddr      = kingpin.Flag("listen", "Address on which to expose metrics.").Default(":9301").String()
	apacheStatusURL = kingpin.Flag("apache-status", "URL to collect Apache status from").Default("").String()
	oodPortalPath   = "/etc/ood/config/ood_portal.yml"
	osHostname      = os.Hostname
	fqdn            = "localhost"
)

type oodPortal struct {
	Servername string `yaml:"servername"`
	Port       string `yaml:"port"`
}

func getFQDN(logger log.Logger) string {
	hostname, err := osHostname()
	if err != nil {
		level.Info(logger).Log("msg", fmt.Sprintf("Unable to determine FQDN: %v", err))
		return fqdn
	}
	return hostname
}

func getApacheStatusURL(logger log.Logger) string {
	defaultApacheStatusURL := "http://" + fqdn + "/server-status"
	var config oodPortal
	var servername, port, apacheStatus string
	_, statErr := os.Stat(oodPortalPath)
	if os.IsNotExist(statErr) {
		level.Info(logger).Log("msg", "File not found, using default Apache status URL", "file", oodPortalPath)
		return defaultApacheStatusURL
	}
	data, err := ioutil.ReadFile(oodPortalPath)
	if err != nil {
		level.Error(logger).Log("msg", fmt.Sprintf("Error reading %s: %v", oodPortalPath, err))
		return defaultApacheStatusURL
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		level.Error(logger).Log("msg", fmt.Sprintf("Error parsing %s: %v", oodPortalPath, err))
		return defaultApacheStatusURL
	}
	level.Debug(logger).Log("msg", fmt.Sprintf("Parsed %s", oodPortalPath), "servername", config.Servername, "port", config.Port, "config", config)
	if config.Servername != "" {
		servername = config.Servername
	} else {
		servername = fqdn
	}
	if config.Port != "" {
		port = config.Port
	} else {
		port = "80"
	}
	if port != "80" {
		apacheStatus = "https://" + servername + "/server-status"
	} else {
		apacheStatus = "http://" + servername + "/server-status"
	}
	return apacheStatus
}

func metricsHandler(logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		collector := collectors.NewCollector(logger)
		collector.Fqdn = getFQDN(logger)
		if *apacheStatusURL == "" {
			collector.ApacheStatus = getApacheStatusURL(logger)
		} else {
			collector.ApacheStatus = *apacheStatusURL
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(collector)
		registry.MustRegister(version.NewCollector("ondemand_exporter"))

		gatherers := prometheus.Gatherers{
			prometheus.DefaultGatherer,
			registry,
		}

		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}

func main() {
	metricsEndpoint := "/metrics"
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("ondemand_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(promlogConfig)
	level.Info(logger).Log("msg", "Starting gpfs_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())
	level.Info(logger).Log("msg", "Starting Server", "address", *listenAddr)

	http.Handle(metricsEndpoint, metricsHandler(logger))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>OnDemand Exporter</title></head>
             <body>
             <h1>OnDemand Exporter</h1>
             <p><a href='` + metricsEndpoint + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	err := http.ListenAndServe(*listenAddr, nil)
	if err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}
}
