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
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	namespace = "ondemand"
)

var (
	execCommand     = exec.Command
	collectDuration = prometheus.NewDesc(prometheus.BuildFQName(namespace, "exporter", "collector_duration_seconds"), "Collector time duration", []string{"collector"}, nil)
)

type Collector struct {
	sync.Mutex
	ApacheStatus    string
	Fqdn            string
	ActivePuns      *prometheus.Desc
	collectFailures prometheus.Counter
	error           prometheus.Gauge
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

func NewCollector() *Collector {
	return &Collector{
		ActivePuns: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "active_puns"), "Active PUNs", nil, nil),
		collectFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "collect_failures_total",
			Help:      "Number of errors while collecting metrics.",
		}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "error",
			Help:      "Indicates if exporter has an error, 0=no errors, 1=errors",
		}),
	}
}

func (c *Collector) collect(ch chan<- prometheus.Metric) error {
	log.Info("Collecting metrics")

	collectTime := time.Now()
	puns, err := getActivePuns()
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.ActivePuns, prometheus.GaugeValue, float64(len(puns)))
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "puns")
	c.error.Set(0)
	c.collectFailures.Add(0)

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func(puns []string) {
		p := NewProcessCollector()
		err := p.collect(puns, ch)
		if err != nil {
			log.Errorf("Error collecting process information: %s", err.Error())
			c.error.Set(1)
		}
		wg.Done()
	}(puns)

	go func() {
		a := NewApacheCollector()
		err := a.collect(c.ApacheStatus, c.Fqdn, ch)
		if err != nil {
			log.Errorf("Error collecting apache information: %s", err.Error())
			c.error.Set(1)
		}
		wg.Done()
	}()
	wg.Wait()
	return nil
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.ActivePuns
	c.error.Describe(ch)
	c.collectFailures.Describe(ch)
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.Lock() // To protect metrics from concurrent collects.
	defer c.Unlock()
	c.error.Collect(ch)
	if err := c.collect(ch); err != nil {
		log.Errorf("Error scraping ondemand: %s", err)
		c.collectFailures.Inc()
	}
	c.collectFailures.Collect(ch)
}
