package main

import (
	"io/ioutil"
	"net/http"
	"os"

    "github.com/OSC/ondemand_exporter/collectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

var (
	listenAddr    = kingpin.Flag("listen", "Address on which to expose metrics.").Default(":9301").String()
	apacheStatusURL  = kingpin.Flag("apache-status", "URL to collect Apache status from").Default("").String()
	oodPortalPath = "/etc/ood/config/ood_portal.yml"
    osHostname    = os.Hostname
	fqdn          = "localhost"
)

type oodPortal struct {
	Servername string `yaml:"servername"`
	Port       string `yaml:"port"`
}

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
	log.Debugf("DATA: %v", string(data))
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Errorf("Error parsing %s: %v", oodPortalPath, err)
		return defaultApacheStatusURL
	}
	log.Debugf("Parsed %s servername=%s port=%s config=%v", oodPortalPath, config.Servername, config.Port, config)
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

func metricsHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
	    collector := collectors.NewCollector()
	    collector.Fqdn = getFQDN()
	    if *apacheStatusURL == "" {
		    collector.ApacheStatus = getApacheStatusURL()
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
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("ondemand_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting ondemand_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting Server: %s", *listenAddr)

	http.Handle(metricsEndpoint, metricsHandler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>OnDemand Exporter</title></head>
             <body>
             <h1>OnDemand Exporter</h1>
             <p><a href='` + metricsEndpoint + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
