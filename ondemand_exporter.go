package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/shirou/gopsutil/process"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

const (
	namespace = "ondemand"
)

var (
	listenAddr    = kingpin.Flag("listen", "Address on which to expose metrics.").Default(":9301").String()
	apacheStatus  = kingpin.Flag("apache-status", "URL to collect Apache status from").Default("").String()
	execCommand   = exec.Command
	osHostname    = os.Hostname
	oodPortalPath = "/etc/ood/config/ood_portal.yml"
	fqdn          = "localhost"
)

type Exporter struct {
	sync.Mutex
	apacheStatus            string
	fqdn                    string
	httpClient              *http.Client
	puns                    []string
	ActivePuns              *prometheus.Desc
	RackApps                *prometheus.Desc
	NodeApps                *prometheus.Desc
	PunCpuPercent           *prometheus.Desc
	PunMemory               *prometheus.Desc
	PunMemoryPercent        *prometheus.Desc
	WebsocketConnections    *prometheus.Desc
	ClientConnections       *prometheus.Desc
	UniqueClientConnections *prometheus.Desc
	UniqueWebsocketClients  *prometheus.Desc
	scrapeFailures          prometheus.Counter
}

type ProcessMetrics struct {
	RackApps         int
	NodeApps         int
	PunCpuPercent    float64
	PunMemoryRSS     uint64
	PunMemoryVMS     uint64
	PunMemoryPercent float32
}
type ApacheMetrics struct {
	WebsocketConnections    int
	ClientConnections       int
	UniqueWebsocketClients  int
	UniqueClientConnections int
}

type oodPortal struct {
	Servername string `yaml:"servername"`
	Port       string `yaml:"port"`
}

type connection map[string]interface{}

func getFQDN() string {
	hostname, err := osHostname()
	if err != nil {
		log.Infof("Unable to determine FQDN: %v", err)
		return fqdn
	}
	return hostname
}

func getApacheStatusURL() string {
	defaultApacheStatusURL := "http://" + fqdn + "/server-status"
	var config oodPortal
	var servername, port, apacheStatus string
	_, statErr := os.Stat(oodPortalPath)
	if os.IsNotExist(statErr) {
		log.Infof("File %s not found, using default Apache status URL", oodPortalPath)
		return defaultApacheStatusURL
	}
	data, err := ioutil.ReadFile(oodPortalPath)
	if err != nil {
		log.Errorf("Error reading %s: %v", oodPortalPath, err)
		return defaultApacheStatusURL
	}
	log.Infof("DATA: %v", string(data))
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Errorf("Error parsing %s: %v", oodPortalPath, err)
		return defaultApacheStatusURL
	}
	log.Infof("Parsed %s servername=%s port=%s config=%v", oodPortalPath, config.Servername, config.Port, config)
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

func sliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if str == s {
			return true
		}
	}
	return false
}

func getActivePuns() ([]string, error) {
	var puns []string
	out, err := execCommand("sudo", "/opt/ood/nginx_stage/sbin/nginx_stage", "nginx_list").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if l == "" {
			continue
		}
		puns = append(puns, l)
	}
	return puns, nil
}

func getProcessMetrics(puns []string) (ProcessMetrics, error) {
	var metrics ProcessMetrics
	rackApps := 0
	nodeApps := 0
	var pun_cpu_percent float64
	var pun_memory_rss uint64
	var pun_memory_vms uint64
	var pun_memory_percent float32
	cores := runtime.NumCPU()
	procs, err := process.Processes()
	if err != nil {
		return ProcessMetrics{}, err
	}
	log.Debugf("Getting process for PUNS: %v", puns)
	for _, proc := range procs {
		user, _ := proc.Username()
		if user == "root" {
			continue
		}
		cmdline, _ := proc.Cmdline()
		if punProc := sliceContains(puns, user); !punProc {
			log.Debugf("Skip proc not owned by PUN user=%s cmdline=%s", user, cmdline)
			continue
		}
		if strings.Contains(cmdline, "rack-loader.rb") {
			rackApps++
		} else if strings.Contains(cmdline, "Passenger NodeApp") {
			nodeApps++
		}
		cpupercent, _ := proc.CPUPercent()
		pun_cpu_percent = pun_cpu_percent + cpupercent
		log.Debugf("PUN user=%s, cmd=%s cpupercent=%f total=%f", user, cmdline, cpupercent, pun_cpu_percent)
		meminfo, err := proc.MemoryInfo()
		if err == nil {
			memrss := meminfo.RSS
			memvms := meminfo.VMS
			pun_memory_rss = pun_memory_rss + memrss
			pun_memory_vms = pun_memory_vms + memvms
		}
		mempercent, err := proc.MemoryPercent()
		if err == nil {
			pun_memory_percent = pun_memory_percent + mempercent
		}
	}
	log.Debugf("APPS rack=%d node=%d", rackApps, nodeApps)
	newcpupercent := pun_cpu_percent / float64(cores)
	log.Debugf("Cores %d New CPU percent: %f", cores, newcpupercent)
	metrics.RackApps = rackApps
	metrics.NodeApps = nodeApps
	metrics.PunCpuPercent = pun_cpu_percent
	metrics.PunMemoryRSS = pun_memory_rss
	metrics.PunMemoryVMS = pun_memory_vms
	metrics.PunMemoryPercent = pun_memory_percent
	return metrics, nil
}

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

func NewExporter() *Exporter {
	return &Exporter{
		ActivePuns:              prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "active_puns"), "Active PUNs", nil, nil),
		RackApps:                prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "rack_apps"), "Number of running Rack apps", nil, nil),
		NodeApps:                prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "node_apps"), "Number of running NodeJS apps", nil, nil),
		PunCpuPercent:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_cpu_percent"), "Percent CPU of all PUNs", nil, nil),
		PunMemory:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_memory"), "Memory used by all PUNs", []string{"type"}, nil),
		PunMemoryPercent:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_memory_percent"), "Percent memory of all PUNs", nil, nil),
		WebsocketConnections:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "websocket_connections"), "Number of websocket connections", nil, nil),
		UniqueWebsocketClients:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_websocket_clients"), "Unique websocket connections", nil, nil),
		ClientConnections:       prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "client_connections"), "Number of client connections", nil, nil),
		UniqueClientConnections: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "unique_client_connections"), "Unique client connections", nil, nil),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrape_failures_total",
			Help:      "Number of errors while collecting metrics.",
		}),
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	log.Info("Collecting metrics")
	puns, err := getActivePuns()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(e.ActivePuns, prometheus.GaugeValue, float64(len(puns)))
	processMetrics, err := getProcessMetrics(puns)
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(e.RackApps, prometheus.GaugeValue, float64(processMetrics.RackApps))
	ch <- prometheus.MustNewConstMetric(e.NodeApps, prometheus.GaugeValue, float64(processMetrics.NodeApps))
	ch <- prometheus.MustNewConstMetric(e.PunCpuPercent, prometheus.GaugeValue, float64(processMetrics.PunCpuPercent))
	ch <- prometheus.MustNewConstMetric(e.PunMemory, prometheus.GaugeValue, float64(processMetrics.PunMemoryRSS), "rss")
	ch <- prometheus.MustNewConstMetric(e.PunMemory, prometheus.GaugeValue, float64(processMetrics.PunMemoryVMS), "vms")
	ch <- prometheus.MustNewConstMetric(e.PunMemoryPercent, prometheus.GaugeValue, float64(processMetrics.PunMemoryPercent))
	apacheMetrics, err := getApacheMetrics(e.apacheStatus, e.fqdn)
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(e.WebsocketConnections, prometheus.GaugeValue, float64(apacheMetrics.WebsocketConnections))
	ch <- prometheus.MustNewConstMetric(e.UniqueWebsocketClients, prometheus.GaugeValue, float64(apacheMetrics.UniqueWebsocketClients))
	ch <- prometheus.MustNewConstMetric(e.ClientConnections, prometheus.GaugeValue, float64(apacheMetrics.ClientConnections))
	ch <- prometheus.MustNewConstMetric(e.UniqueClientConnections, prometheus.GaugeValue, float64(apacheMetrics.UniqueClientConnections))
	return nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.scrapeFailures.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.Lock() // To protect metrics from concurrent collects.
	defer e.Unlock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping ondemand: %s", err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}
	return
}

func main() {
	metricsEndpoint := "/metrics"
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("ondemand_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting apache_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting Server: %s", *listenAddr)

	fqdn = getFQDN()

	exporter := NewExporter()
	if *apacheStatus == "" {
		exporter.apacheStatus = getApacheStatusURL()
	} else {
		exporter.apacheStatus = *apacheStatus
	}

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("ondemand_exporter"))

	http.Handle(metricsEndpoint, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>OnDemand Exporter</title></head>
             <body>
             <h1>Apache Exporter</h1>
             <p><a href='` + metricsEndpoint + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
