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
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	processTimeout = kingpin.Flag("collector.process.timeout", "Timeout for process collection").Default("10").Envar("PROCESS_TIMEOUT").Int()
	procFS         = "/proc"
)

type ProcessCollector struct {
	RackApps         *prometheus.Desc
	NodeApps         *prometheus.Desc
	PunCpuTime       *prometheus.Desc
	PunMemory        *prometheus.Desc
	PunMemoryPercent *prometheus.Desc
	logger           log.Logger
}

type ProcessMetrics struct {
	RackApps         float64
	NodeApps         float64
	PunCpuTime       float64
	PunMemoryRSS     float64
	PunMemoryVMS     float64
	PunMemoryPercent float64
}

func getProcessMetrics(puns []string, logger log.Logger) (ProcessMetrics, error) {
	var metrics ProcessMetrics
	var rackApps, nodeApps float64
	var pun_cpu_time float64
	var pun_memory_rss, pun_memory_vms float64
	procfs, err := procfs.NewFS(procFS)
	if err != nil {
		return ProcessMetrics{}, err
	}
	meminfo, err := procfs.Meminfo()
	if err != nil {
		return ProcessMetrics{}, err
	}
	procs, err := procfs.AllProcs()
	if err != nil {
		return ProcessMetrics{}, err
	}
	level.Debug(logger).Log("msg", "Getting process for PUNS", "puns", strings.Join(puns, ","))
	for _, proc := range procs {
		procCmdLine, _ := proc.CmdLine()
		cmdline := strings.Join(procCmdLine, " ")
		status, err := proc.NewStatus()
		if err != nil {
			level.Debug(logger).Log("msg", "Unable to get process status", "pid", proc.PID, "cmdline", cmdline)
			continue
		}
		uid := status.UIDs[0]
		stat, err := proc.Stat()
		if err != nil {
			level.Debug(logger).Log("msg", "Unable to get PUN proc stat", "pid", proc.PID, "uid", uid, "cmdline", cmdline)
			continue
		}
		if uid == "0" {
			continue
		}
		if !sliceContains(puns, uid) {
			level.Debug(logger).Log("msg", "Skip PID that does not belong to PUN", "pid", proc.PID, "uid", uid, "cmdline", cmdline)
			continue
		}
		if strings.Contains(cmdline, "rack-loader.rb") {
			rackApps++
		} else if strings.Contains(cmdline, "Passenger NodeApp") {
			nodeApps++
		}
		level.Debug(logger).Log("msg", "Collecting PUN proc stat", "pid", proc.PID, "uid", uid,
			"cputime", stat.CPUTime(), "rss", stat.ResidentMemory(), "vms", stat.VirtualMemory())
		pun_cpu_time = pun_cpu_time + stat.CPUTime()
		pun_memory_rss = pun_memory_rss + float64(stat.ResidentMemory())
		pun_memory_vms = pun_memory_vms + float64(stat.VirtualMemory())
	}
	metrics.RackApps = rackApps
	metrics.NodeApps = nodeApps
	metrics.PunCpuTime = pun_cpu_time
	metrics.PunMemoryRSS = pun_memory_rss
	metrics.PunMemoryVMS = pun_memory_vms
	metrics.PunMemoryPercent = 100 * (float64(pun_memory_rss) / (float64(*meminfo.MemTotal) * 1024.0))
	return metrics, nil
}

func NewProcessCollector(logger log.Logger) *ProcessCollector {
	return &ProcessCollector{
		logger:           logger,
		RackApps:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "rack_apps"), "Number of running Rack apps", nil, nil),
		NodeApps:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "node_apps"), "Number of running NodeJS apps", nil, nil),
		PunCpuTime:       prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "pun_cpu_time"), "CPU time of all PUNs", nil, nil),
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
	ch <- prometheus.MustNewConstMetric(c.RackApps, prometheus.GaugeValue, processMetrics.RackApps)
	ch <- prometheus.MustNewConstMetric(c.NodeApps, prometheus.GaugeValue, processMetrics.NodeApps)
	ch <- prometheus.MustNewConstMetric(c.PunCpuTime, prometheus.GaugeValue, processMetrics.PunCpuTime)
	ch <- prometheus.MustNewConstMetric(c.PunMemory, prometheus.GaugeValue, processMetrics.PunMemoryRSS, "rss")
	ch <- prometheus.MustNewConstMetric(c.PunMemory, prometheus.GaugeValue, processMetrics.PunMemoryVMS, "vms")
	ch <- prometheus.MustNewConstMetric(c.PunMemoryPercent, prometheus.GaugeValue, processMetrics.PunMemoryPercent)
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "process")
	return nil
}
