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
	apacheStatus              string
	fqdn                      string
	httpClient                *http.Client
	puns                      []string
	active_puns               prometheus.Gauge
	rack_apps                 prometheus.Gauge
	node_apps                 prometheus.Gauge
	pun_cpu_time              *prometheus.CounterVec
	pun_cpu_percent           prometheus.Gauge
	pun_memory                *prometheus.GaugeVec
	pun_memory_percent        prometheus.Gauge
	websocket_connections     prometheus.Gauge
	client_connections        prometheus.Gauge
	unique_client_connections prometheus.Gauge
	unique_websocket_clients  prometheus.Gauge
	scrapeFailures            prometheus.Counter
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

func NewExporter() *Exporter {
	return &Exporter{
		active_puns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_puns",
			Help:      "Active PUNs",
		}),
		rack_apps: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "rack_apps",
			Help:      "Number of running Rack apps",
		}),
		node_apps: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "node_apps",
			Help:      "Number of running NodeJS apps",
		}),
		pun_cpu_time: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pun_cpu_time",
			Help:      "CPU time of all PUNs",
		}, []string{"mode"}),
		pun_cpu_percent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "pun_cpu_percent",
			Help:      "Percent CPU of all PUNs",
		}),
		pun_memory: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "pun_memory",
			Help:      "Memory used by all PUNs",
		}, []string{"type"}),
		pun_memory_percent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "pun_memory_percent",
			Help:      "Percent memory of all PUNs",
		}),
		websocket_connections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "websocket_connections",
			Help:      "Number of Websocket Connections",
		}),
		unique_websocket_clients: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "unique_websocket_clients",
			Help:      "Number of unique Websocket Connections",
		}),
		client_connections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "client_connections",
			Help:      "Number of client connections",
		}),
		unique_client_connections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "unique_client_connections",
			Help:      "Number of unique client Connections",
		}),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while collecting metrics.",
		}),
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (e *Exporter) getProcessMetrics() error {
	rackApps := 0
	nodeApps := 0
	var pun_cpu_percent float64
	var pun_memory_rss uint64
	var pun_memory_vms uint64
	var pun_memory_percent float32
	cores := runtime.NumCPU()
	procs, err := process.Processes()
	if err != nil {
		return err
	}
	log.Debugf("Getting process for PUNS: %v", e.puns)
	for _, proc := range procs {
		user, _ := proc.Username()
		if user == "root" {
			continue
		}
		cmdline, _ := proc.Cmdline()
		if punProc := sliceContains(e.puns, user); !punProc {
			log.Debugf("Skip proc not owned by PUN user=%s cmdline=%s", user, cmdline)
			continue
		}
		if strings.Contains(cmdline, "rack-loader.rb") {
			rackApps++
		} else if strings.Contains(cmdline, "Passenger NodeApp") {
			nodeApps++
		}
		cputime, err := proc.Times()
		if err == nil {
			cpuuser := cputime.User
			cpusys := cputime.System
			e.pun_cpu_time.WithLabelValues("user").Add(cpuuser)
			e.pun_cpu_time.WithLabelValues("system").Add(cpusys)
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
	e.active_puns.Set(float64(len(e.puns)))
	e.rack_apps.Set(float64(rackApps))
	e.node_apps.Set(float64(nodeApps))
	e.pun_cpu_percent.Set(newcpupercent)
	e.pun_memory.WithLabelValues("rss").Set(float64(pun_memory_rss))
	e.pun_memory.WithLabelValues("vms").Set(float64(pun_memory_vms))
	e.pun_memory_percent.Set(float64(pun_memory_percent))
	return nil
}

func (e *Exporter) getApacheMetrics() error {
	log.Infof("GET: %s", e.apacheStatus)
	resp, err := http.Get(e.apacheStatus)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			data = []byte(err.Error())
		}
		return fmt.Errorf("Status %s (%d): %s", resp.Status, resp.StatusCode, data)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}
	var headers []string
	var connections []connection
	doc.Find("table").First().Each(func(index int, tablehtml *goquery.Selection) {
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

	})
	var websocket_connections, client_connections int
	var clients []string
	var unique_client_connections []string
	var unique_websocket_clients []string
	localClients := []string{e.fqdn, "localhost", "127.0.0.1"}
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
	e.websocket_connections.Set(float64(websocket_connections))
	e.client_connections.Set(float64(client_connections))
	e.unique_client_connections.Set(float64(len(unique_client_connections)))
	e.unique_websocket_clients.Set(float64(len(unique_websocket_clients)))
	return nil
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	log.Info("Collecting metrics")
	defer e.active_puns.Collect(ch)
	defer e.rack_apps.Collect(ch)
	defer e.node_apps.Collect(ch)
	defer e.pun_cpu_time.Collect(ch)
	defer e.pun_cpu_percent.Collect(ch)
	defer e.pun_memory.Collect(ch)
	defer e.pun_memory_percent.Collect(ch)
	defer e.websocket_connections.Collect(ch)
	defer e.client_connections.Collect(ch)
	defer e.unique_client_connections.Collect(ch)
	defer e.unique_websocket_clients.Collect(ch)
	puns, err := getActivePuns()
	if err != nil {
		return err
	}
	e.active_puns.Set(float64(len(puns)))
	e.puns = puns
	if err := e.getProcessMetrics(); err != nil {
		return err
	}
	if err := e.getApacheMetrics(); err != nil {
		return err
	}
	return nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.active_puns.Describe(ch)
	e.rack_apps.Describe(ch)
	e.node_apps.Describe(ch)
	e.pun_cpu_time.Describe(ch)
	e.pun_cpu_percent.Describe(ch)
	e.pun_memory.Describe(ch)
	e.pun_memory_percent.Describe(ch)
	e.websocket_connections.Describe(ch)
	e.client_connections.Describe(ch)
	e.unique_client_connections.Describe(ch)
	e.unique_websocket_clients.Describe(ch)
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
