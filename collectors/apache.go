package collectors

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type ApacheCollector struct {
	httpClient              *http.Client
	WebsocketConnections    *prometheus.Desc
	ClientConnections       *prometheus.Desc
	UniqueClientConnections *prometheus.Desc
	UniqueWebsocketClients  *prometheus.Desc
}

type ApacheMetrics struct {
	WebsocketConnections    int
	ClientConnections       int
	UniqueWebsocketClients  int
	UniqueClientConnections int
}

type connection map[string]interface{}

func getApacheMetrics(apacheStatus string, fqdn string) (ApacheMetrics, error) {
	var metrics ApacheMetrics
	log.Infof("GET: %s", apacheStatus)
	resp, err := http.Get(apacheStatus)
	if err != nil {
		return metrics, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, err := ioutil.ReadAll(resp.Body)
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
	var clients []string
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
			log.Debugf("Skip request: %s", request)
			continue
		}
		clients = append(clients, client)
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

func NewApacheCollector() *ApacheCollector {
	return &ApacheCollector{
		WebsocketConnections:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "websocket_connections"), "Number of websocket connections", nil, nil),
		UniqueWebsocketClients:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_websocket_clients"), "Unique websocket connections", nil, nil),
		ClientConnections:       prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "client_connections"), "Number of client connections", nil, nil),
		UniqueClientConnections: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_client_connections"), "Unique client connections", nil, nil),
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (c *ApacheCollector) collect(apacheStatus string, fqdn string, ch chan<- prometheus.Metric) error {
	log.Info("Collecting apache metrics")
	collectTime := time.Now()
	apacheMetrics, err := getApacheMetrics(apacheStatus, fqdn)
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
