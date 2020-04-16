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
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/process"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	processTimeout = kingpin.Flag("collector.process.timeout", "Timeout for process collection").Default("10").Int()
)

type ProcessCollector struct {
	RackApps         *prometheus.Desc
	NodeApps         *prometheus.Desc
	PunCpuPercent    *prometheus.Desc
	PunMemory        *prometheus.Desc
	PunMemoryPercent *prometheus.Desc
	logger           log.Logger
}

type ProcessMetrics struct {
	RackApps         int
	NodeApps         int
	PunCpuPercent    float64
	PunMemoryRSS     uint64
	PunMemoryVMS     uint64
	PunMemoryPercent float32
}

func getProcessMetrics(puns []string, logger log.Logger) (ProcessMetrics, error) {
	var metrics ProcessMetrics
	rackApps := 0
	nodeApps := 0
	var pun_cpu_percent float64
	var pun_memory_rss uint64
	var pun_memory_vms uint64
	var pun_memory_percent float32
	procs, err := process.Processes()
	if err != nil {
		return ProcessMetrics{}, err
	}
	level.Debug(logger).Log("msg", "Getting process for PUNS", "puns", puns)
	for _, proc := range procs {
		user, _ := proc.Username()
		if user == "root" {
			continue
		}
		cmdline, _ := proc.Cmdline()
		if punProc := sliceContains(puns, user); !punProc {
			level.Debug(logger).Log("msg", "Skip proc not owned by PUN", "user", user, "cmdline", cmdline)
			continue
		}
		if strings.Contains(cmdline, "rack-loader.rb") {
			rackApps++
		} else if strings.Contains(cmdline, "Passenger NodeApp") {
			nodeApps++
		}
		cpupercent, _ := proc.CPUPercent()
		pun_cpu_percent = pun_cpu_percent + cpupercent
		level.Debug(logger).Log("msg", "PUN", "user", user, "cmd", cmdline, "cpupercent", cpupercent, "total", pun_cpu_percent)
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
	level.Debug(logger).Log("msg", "APPS", "rack", rackApps, "node", nodeApps)
	newcpupercent := pun_cpu_percent / float64(cores())
	level.Debug(logger).Log("msg", fmt.Sprintf("Cores %d New CPU percent: %f", cores(), newcpupercent))
	metrics.RackApps = rackApps
	metrics.NodeApps = nodeApps
	metrics.PunCpuPercent = pun_cpu_percent
	metrics.PunMemoryRSS = pun_memory_rss
	metrics.PunMemoryVMS = pun_memory_vms
	metrics.PunMemoryPercent = pun_memory_percent
	return metrics, nil
}

func NewProcessCollector(logger log.Logger) *ProcessCollector {
	return &ProcessCollector{
		logger:           logger,
		RackApps:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "rack_apps"), "Number of running Rack apps", nil, nil),
		NodeApps:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "node_apps"), "Number of running NodeJS apps", nil, nil),
		PunCpuPercent:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_cpu_percent"), "Percent CPU of all PUNs", nil, nil),
		PunMemory:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_memory"), "Memory used by all PUNs", []string{"type"}, nil),
		PunMemoryPercent: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_memory_percent"), "Percent memory of all PUNs", nil, nil),
	}
}

func (c *ProcessCollector) collect(puns []string, ch chan<- prometheus.Metric) error {
	level.Debug(c.logger).Log("msg", "Collecting process metrics")
	collectTime := time.Now()

	c1 := make(chan int, 1)
	timeout := false
	var processMetrics ProcessMetrics
	var err error
	go func() {
		processMetrics, err = getProcessMetrics(puns, c.logger)
		if !timeout {
			c1 <- 1
		}
	}()
	select {
	case <-c1:
	case <-time.After(time.Duration(*processTimeout) * time.Second):
		timeout = true
		close(c1)
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 1, "process")
		level.Error(c.logger).Log("msg", "Timeout collecting process information")
		return nil
	}
	close(c1)
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 0, "process")
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
