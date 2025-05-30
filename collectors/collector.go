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
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "ondemand"
)

var (
	punsTimeout     = kingpin.Flag("collector.puns.timeout", "Timeout for collecting PUNs").Default("10").Envar("PUNS_TIMEOUT").Int()
	oodPortalPath   = "/etc/ood/config/ood_portal.yml"
	execCommand     = exec.CommandContext
	timeNow         = getTimeNow
	cores           = getCores
	collectDuration = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "collect_duration_seconds"),
		"Collector time duration", []string{"collector"}, nil)
	collecTimeout = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "collect_timeout"),
		"Indicates the collector timed out", []string{"collector"}, nil)
	collecError = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "exporter", "collect_error"),
		"Indicates the collector had an error", []string{"collector"}, nil)
)

type Collector struct {
	sync.Mutex
	ApacheStatus string
	Fqdn         string
	ActivePuns   *prometheus.Desc
	logger       *slog.Logger
}

type oodPortal struct {
	Servername string `yaml:"servername"`
	Port       string `yaml:"port"`
}

func sliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if str == s {
			return true
		}
	}
	return false
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getTimeNow() time.Time {
	return time.Now()
}

func getCores() int {
	return runtime.NumCPU()
}

func getActivePuns(ctx context.Context, logger *slog.Logger) ([]string, []string, error) {
	var puns []string
	var punUIDs []string
	out, err := execCommand(ctx, "sudo", "/opt/ood/nginx_stage/sbin/nginx_stage", "nginx_list").Output()
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if l == "" {
			continue
		}
		puns = append(puns, l)
		user, err := user.Lookup(l)
		if err != nil {
			logger.Error("Unable to lookup PUN username", "pun", l)
			continue
		}
		punUIDs = append(punUIDs, user.Uid)
	}
	logger.Debug("Found PUNs", "puns", strings.Join(puns, ","), "punUIDs", strings.Join(punUIDs, ","))
	return puns, punUIDs, nil
}

func NewCollector(logger *slog.Logger) *Collector {
	return &Collector{
		logger:     logger,
		ActivePuns: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "active_puns"), "Active PUNs", nil, nil),
	}
}

func (c *Collector) collect(ch chan<- prometheus.Metric) error {
	c.logger.Debug("Collecting metrics")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*punsTimeout)*time.Second)
	defer cancel()
	collectTime := time.Now()
	puns, punUIDs, err := getActivePuns(ctx, c.logger)
	if ctx.Err() == context.DeadlineExceeded {
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 1, "puns")
		c.logger.Error("Timeout collecting PUNs")
		return nil
	}
	ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, 0, "puns")
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 1, "puns")
		c.logger.Error(err.Error())
		return nil
	}
	ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 0, "puns")
	ch <- prometheus.MustNewConstMetric(c.ActivePuns, prometheus.GaugeValue, float64(len(puns)))
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "puns")

	wg := &sync.WaitGroup{}
	wg.Add(3)

	go func(puns []string) {
		p := NewProcessCollector(c.logger.With("collector", "process"))
		err := p.collect(puns, ch)
		if err != nil {
			c.logger.Error("Error collecting process information", "err", err)
			ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 1, "process")
		} else {
			ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 0, "process")
		}
		wg.Done()
	}(punUIDs)

	go func() {
		a := NewApacheCollector(c.logger)
		err := a.collect(ch)
		if err != nil {
			c.logger.Error("Error collecting apache information", "err", err)
			ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 1, "apache")
		} else {
			ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 0, "apache")
		}
		wg.Done()
	}()

	go func(puns []string) {
		p := NewPassengerCollector(c.logger)
		err := p.collect(puns, ch)
		if err != nil {
			c.logger.Error("Error collecting passenger information", "err", err)
			ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 1, "passenger")
		} else {
			ch <- prometheus.MustNewConstMetric(collecError, prometheus.GaugeValue, 0, "passenger")
		}
		wg.Done()
	}(punUIDs)
	wg.Wait()
	return nil
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.ActivePuns
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.Lock() // To protect metrics from concurrent collects.
	defer c.Unlock()
	if err := c.collect(ch); err != nil {
		c.logger.Error("Error scraping ondemand", "err", err)
	}
}
