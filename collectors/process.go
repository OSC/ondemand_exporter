package collectors

import (
	"runtime"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/shirou/gopsutil/process"
)

type ProcessCollector struct {
	RackApps         *prometheus.Desc
	NodeApps         *prometheus.Desc
	PunCpuPercent    *prometheus.Desc
	PunMemory        *prometheus.Desc
	PunMemoryPercent *prometheus.Desc
}

type ProcessMetrics struct {
	RackApps         int
	NodeApps         int
	PunCpuPercent    float64
	PunMemoryRSS     uint64
	PunMemoryVMS     uint64
	PunMemoryPercent float32
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

func NewProcessCollector() *ProcessCollector {
	return &ProcessCollector{
		RackApps:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "rack_apps"), "Number of running Rack apps", nil, nil),
		NodeApps:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "node_apps"), "Number of running NodeJS apps", nil, nil),
		PunCpuPercent:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_cpu_percent"), "Percent CPU of all PUNs", nil, nil),
		PunMemory:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_memory"), "Memory used by all PUNs", []string{"type"}, nil),
		PunMemoryPercent: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_memory_percent"), "Percent memory of all PUNs", nil, nil),
	}
}

func (c *ProcessCollector) collect(puns []string, ch chan<- prometheus.Metric) error {
	log.Info("Collecting process metrics")
	collectTime := time.Now()
	processMetrics, err := getProcessMetrics(puns)
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.RackApps, prometheus.GaugeValue, float64(processMetrics.RackApps))
	ch <- prometheus.MustNewConstMetric(c.NodeApps, prometheus.GaugeValue, float64(processMetrics.NodeApps))
	ch <- prometheus.MustNewConstMetric(c.PunCpuPercent, prometheus.GaugeValue, float64(processMetrics.PunCpuPercent))
	ch <- prometheus.MustNewConstMetric(c.PunMemory, prometheus.GaugeValue, float64(processMetrics.PunMemoryRSS), "rss")
	ch <- prometheus.MustNewConstMetric(c.PunMemory, prometheus.GaugeValue, float64(processMetrics.PunMemoryVMS), "vms")
	ch <- prometheus.MustNewConstMetric(c.PunMemoryPercent, prometheus.GaugeValue, float64(processMetrics.PunMemoryPercent))
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "process")
	return nil
}
