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
	"log/slog"
	"net/http"
	"os"

	"github.com/OSC/ondemand_exporter/collectors"
	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
)

var (
	listenAddr = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9301").Envar("LISTEN_ADDRESS").String()
)

func metricsHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		collector := collectors.NewCollector(logger)
		registry := prometheus.NewRegistry()
		registry.MustRegister(collector)
		registry.MustRegister(versioncollector.NewCollector("ondemand_exporter"))

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
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("ondemand_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)
	logger.Info("Starting ondemand_exporter", "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())
	logger.Info("Starting Server", "address", *listenAddr)

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
		logger.Error(err.Error())
		os.Exit(1)
	}
}
