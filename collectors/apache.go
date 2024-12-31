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

package collectors

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

var (
	apacheStatusURL = kingpin.Flag("collector.apache.status-url", "URL to collect Apache status from").Default("").Envar("APACHE_STATUS_URL").String()
	apacheTimeout   = kingpin.Flag("collector.apache.timeout", "Timeout for collecting Apache metrics").Default("10").Envar("APACHE_TIMEOUT").Int()
	osHostname      = os.Hostname
	fqdn            = "localhost"
)

type ApacheCollector struct {
	WebsocketConnections    *prometheus.Desc
	ClientConnections       *prometheus.Desc
	UniqueClientConnections *prometheus.Desc
	UniqueWebsocketClients  *prometheus.Desc
	logger                  *slog.Logger
}

type ApacheMetrics struct {
	WebsocketConnections    int
	ClientConnections       int
	UniqueWebsocketClients  int
	UniqueClientConnections int
}

type connection map[string]interface{}

func getFQDN(logger *slog.Logger) string {
	hostname, err := osHostname()
	if err != nil {
		logger.Info(fmt.Sprintf("Unable to determine FQDN: %v", err))
		return fqdn
	}
	return hostname
}

func getApacheStatusURL(logger *slog.Logger) string {
	defaultApacheStatusURL := "http://" + fqdn + "/server-status"
	var config oodPortal
	var servername, port, apacheStatus string
	_, statErr := os.Stat(oodPortalPath)
	if os.IsNotExist(statErr) {
		logger.Info("File not found, using default Apache status URL", "file", oodPortalPath)
		return defaultApacheStatusURL
	}
	data, err := os.ReadFile(oodPortalPath)
	if err != nil {
		logger.Error(fmt.Sprintf("Error reading %s: %v", oodPortalPath, err))
		return defaultApacheStatusURL
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		logger.Error(fmt.Sprintf("Error parsing %s: %v", oodPortalPath, err))
		return defaultApacheStatusURL
	}
	logger.Debug(fmt.Sprintf("Parsed %s", oodPortalPath), "servername", config.Servername, "port", config.Port, "config", config)
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

func getApacheMetrics(apacheStatus string, fqdn string, ctx context.Context, logger *slog.Logger) (ApacheMetrics, error) {
	var metrics ApacheMetrics
	req, err := http.NewRequest("GET", apacheStatus, nil)
	if err != nil {
		return metrics, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return metrics, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			data = []byte(err.Error())
		}
		return metrics, fmt.Errorf("Status %s (%d): %s", resp.Status, resp.StatusCode, data)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return metrics, err
	}
	var headers []string
	var connections []connection
	var tableFound bool
	doc.Find("table").EachWithBreak(func(index int, tablehtml *goquery.Selection) bool {
		tablehtml.Find("th").First().Each(func(indextr int, tableheading *goquery.Selection) {
			if tableheading.Text() == "Srv" {
				tableFound = true
			}
		})
		if tableFound {
			tablehtml.Find("tr").Each(func(indextr int, rowhtml *goquery.Selection) {
				conn := connection{}
				rowhtml.Find("th").Each(func(indexth int, tableheading *goquery.Selection) {
					headers = append(headers, tableheading.Text())
				})
				rowhtml.Find("td").Each(func(indextd int, tablecell *goquery.Selection) {
					key := headers[indextd]
					value := tablecell.Text()
					conn[key] = value
				})
				connections = append(connections, conn)
			})
			return false
		}
		return true
	})
	var websocket_connections, client_connections int
	var unique_client_connections []string
	var unique_websocket_clients []string
	localClients := []string{fqdn, "localhost", "127.0.0.1"}
	for _, c := range connections {
		if _, ok := c["Client"]; !ok {
			continue
		}
		if _, ok := c["Request"]; !ok {
			continue
		}
		request := c["Request"].(string)
		client := c["Client"].(string)
		if !strings.Contains(request, "/node/") &&
			!strings.Contains(request, "/rnode/") &&
			!strings.Contains(request, "websockify") &&
			!strings.Contains(request, "/pun/") &&
			!strings.Contains(request, "/nginx/") &&
			!strings.Contains(request, "/oidc") {
			//level.Debug(logger).Log("msg", "Skip request", "request", request)
			continue
		}
		if strings.Contains(request, "/node/") || strings.Contains(request, "/rnode/") || strings.Contains(request, "websockify") {
			websocket_connections++
			if contains := sliceContains(unique_websocket_clients, client); !contains {
				unique_websocket_clients = append(unique_websocket_clients, client)
			}
		}
		if localClient := sliceContains(localClients, client); !localClient {
			client_connections++
			if contains := sliceContains(unique_client_connections, client); !contains {
				unique_client_connections = append(unique_client_connections, client)
			}
		}
	}
	metrics.WebsocketConnections = websocket_connections
	metrics.UniqueWebsocketClients = len(unique_websocket_clients)
	metrics.ClientConnections = client_connections
	metrics.UniqueClientConnections = len(unique_client_connections)
	return metrics, nil
}

func NewApacheCollector(logger *slog.Logger) *ApacheCollector {
	logger.LogAttrs(context.Background(), slog.LevelInfo, "apache collector", slog.String("collector", "apache"))
	return &ApacheCollector{
		logger:                  logger,
		WebsocketConnections:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "websocket_connections"), "Number of websocket connections", nil, nil),
		UniqueWebsocketClients:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_websocket_clients"), "Unique websocket connections", nil, nil),
		ClientConnections:       prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "client_connections"), "Number of client connections", nil, nil),
		UniqueClientConnections: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_client_connections"), "Unique client connections", nil, nil),
	}
}

func (c *ApacheCollector) collect(ch chan<- prometheus.Metric) error {
	var apacheStatus string
	fqdn = getFQDN(c.logger)
	if *apacheStatusURL == "" {
		apacheStatus = getApacheStatusURL(c.logger)
	} else {
		apacheStatus = *apacheStatusURL
	}
	c.logger.Debug("Collecting apache metrics")
	collectTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*apacheTimeout)*time.Second)
	defer cancel()
	apacheMetrics, err := getApacheMetrics(apacheStatus, fqdn, ctx, c.logger)
	if ctx.Err() == context.DeadlineExceeded {
		c.logger.Error("Timeout requesting Apache metrics")
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 1, "apache")
		return nil
	}
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 0, "apache")
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.WebsocketConnections, prometheus.GaugeValue, float64(apacheMetrics.WebsocketConnections))
	ch <- prometheus.MustNewConstMetric(c.UniqueWebsocketClients, prometheus.GaugeValue, float64(apacheMetrics.UniqueWebsocketClients))
	ch <- prometheus.MustNewConstMetric(c.ClientConnections, prometheus.GaugeValue, float64(apacheMetrics.ClientConnections))
	ch <- prometheus.MustNewConstMetric(c.UniqueClientConnections, prometheus.GaugeValue, float64(apacheMetrics.UniqueClientConnections))
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "apache")
	return nil
}
