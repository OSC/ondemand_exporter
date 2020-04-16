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
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/html/charset"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	microsecondsPerSecond = 1000000
)

var (
	passengerTimeout            = kingpin.Flag("collector.passenger.timeout", "Timeout for collecting Passenger metrics").Default("30").Int()
	passengerStatusPath         = kingpin.Flag("path.passenger-status", "Path to OnDemand passenger-status").Default("/usr/local/sbin/ondemand-passenger-status").String()
	passengerStatusExec         = passengerStatus
	passengerStatusExecInstance = passengerStatus
	passengerMetricMutex        = sync.RWMutex{}
)

type PassengerCollector struct {
	Instances  *prometheus.Desc
	Count      *prometheus.Desc
	ProcCount  *prometheus.Desc
	RSS        *prometheus.Desc
	CPU        *prometheus.Desc
	RealMemory *prometheus.Desc
	Requests   *prometheus.Desc
	AvgRuntime *prometheus.Desc
	logger     log.Logger
}

type PassengerAppMetrics struct {
	Name              string
	Count             int
	ProcCount         int
	Processes         []PassengerProcessMetrics
	RSS               int
	CPU               int
	RealMemory        int
	RequestsProcessed int
	Runtime           int64
}
type PassengerProcessMetrics struct {
	RSS               int
	CPU               int
	RealMemory        int
	RequestsProcessed int
	Runtime           int64
}

func NewPassengerCollector(logger log.Logger) *PassengerCollector {
	return &PassengerCollector{
		logger:     logger,
		Instances:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger", "instances"), "Number of Passenger instances", nil, nil),
		Count:      prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger_app", "count"), "Count of passenger instances of an app", []string{"app"}, nil),
		ProcCount:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger_app", "processes"), "Process count of an app", []string{"app"}, nil),
		RSS:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger_app", "rss_bytes"), "RSS of passenger apps", []string{"app"}, nil),
		RealMemory: prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger_app", "real_memory_bytes"), "Real memory of passenger apps", []string{"app"}, nil),
		CPU:        prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger_app", "cpu_percent"), "CPU percent of passenger apps", []string{"app"}, nil),
		Requests:   prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger_app", "requests_total"), "Requests made to passenger apps", []string{"app"}, nil),
		AvgRuntime: prometheus.NewDesc(prometheus.BuildFQName(namespace, "passenger_app", "average_runtime_seconds"), "Average runtime in seconds of passenger apps", []string{"app"}, nil),
	}
}

func (c *PassengerCollector) collect(puns []string, ch chan<- prometheus.Metric) error {
	level.Debug(c.logger).Log("msg", "Collecting passenger metrics")
	if !fileExists(*passengerStatusPath) {
		return fmt.Errorf("%s not found", *passengerStatusPath)
	}
	collectTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*passengerTimeout)*time.Second)
	defer cancel()
	instances, err := c.getInstances(ctx)
	if err != nil {
		return err
	}
	metrics, errors := c.getInstancesMetrics(ctx, instances)
	if errors != nil {
		err := ""
		for _, e := range errors {
			err = err + " " + e.Error()
		}
		return fmt.Errorf("%s", err)
	}
	appMetrics := make(map[string]PassengerAppMetrics)
	for _, m := range metrics {
		var metric PassengerAppMetrics
		am, ok := appMetrics[m.Name]
		if ok {
			level.Debug(c.logger).Log("msg", "Existing app metric", "app", m.Name, "processes", len(m.Processes))
			metric = am
		} else {
			level.Debug(c.logger).Log("msg", "New app metric", "app", m.Name, "processes", len(m.Processes))
			metric = m
		}
		metric.Count++
		level.Debug(c.logger).Log("msg", "App count", "app", m.Name, "count", metric.Count)
		for _, p := range m.Processes {
			metric.ProcCount++
			metric.RSS = metric.RSS + p.RSS
			metric.RealMemory = metric.RealMemory + p.RealMemory
			metric.CPU = metric.CPU + p.CPU
			metric.RequestsProcessed = metric.RequestsProcessed + p.RequestsProcessed
			metric.Runtime = metric.Runtime + p.Runtime
		}
		appMetrics[m.Name] = metric
	}
	for name, metric := range appMetrics {
		ch <- prometheus.MustNewConstMetric(c.Count, prometheus.GaugeValue, float64(metric.Count), name)
		ch <- prometheus.MustNewConstMetric(c.ProcCount, prometheus.GaugeValue, float64(metric.ProcCount), name)
		ch <- prometheus.MustNewConstMetric(c.RSS, prometheus.GaugeValue, float64(metric.RSS), name)
		ch <- prometheus.MustNewConstMetric(c.RealMemory, prometheus.GaugeValue, float64(metric.RealMemory), name)
		ch <- prometheus.MustNewConstMetric(c.CPU, prometheus.GaugeValue, float64(metric.CPU), name)
		ch <- prometheus.MustNewConstMetric(c.Requests, prometheus.CounterValue, float64(metric.RequestsProcessed), name)
		var runtime float64
		if metric.ProcCount > 0 {
			runtime = float64(metric.Runtime / int64(metric.ProcCount))
		} else {
			runtime = 0
		}
		ch <- prometheus.MustNewConstMetric(c.AvgRuntime, prometheus.GaugeValue, runtime, name)
	}
	ch <- prometheus.MustNewConstMetric(c.Instances, prometheus.GaugeValue, float64(len(instances)))
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "passenger")
	return nil
}

func (c *PassengerCollector) getInstances(ctx context.Context) ([]string, error) {
	out, err := passengerStatusExec(ctx, "", c.logger)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	var instances []string
	var pastHeader bool
	for _, l := range lines {
		if strings.HasPrefix(l, "---") {
			pastHeader = true
		}
		if !pastHeader {
			continue
		}
		items := strings.Fields(l)
		if len(items) != 4 {
			continue
		}
		instances = append(instances, items[0])
	}
	return instances, nil
}

func (c *PassengerCollector) getInstancesMetrics(ctx context.Context, instances []string) ([]PassengerAppMetrics, []error) {
	wg := &sync.WaitGroup{}
	var metrics []PassengerAppMetrics
	var errors []error
	wg.Add(len(instances))
	for _, instance := range instances {
		go func(inst string) {
			level.Debug(c.logger).Log("msg", "Collecting passenger instance metrics", "instance", inst)
			defer wg.Done()
			m, err := c.getMetrics(ctx, inst)
			if err != nil {
				level.Error(c.logger).Log("msg", fmt.Sprintf("Error collecting %s instance metrics", inst), "err", err)
				passengerMetricMutex.Lock()
				errors = append(errors, err)
				passengerMetricMutex.Unlock()
				return
			}
			level.Debug(c.logger).Log("msg", "DONE Collecting passenger instance metrics", "instance", inst)
			passengerMetricMutex.Lock()
			metrics = append(metrics, m...)
			passengerMetricMutex.Unlock()
		}(instance)
	}
	wg.Wait()
	return metrics, errors
}

func (c *PassengerCollector) getMetrics(ctx context.Context, instance string) ([]PassengerAppMetrics, error) {
	now := timeNow().Unix()
	level.Debug(c.logger).Log("msg", "NOW", "now", now)
	var metrics []PassengerAppMetrics
	out, err := passengerStatusExecInstance(ctx, instance, c.logger)
	if err != nil {
		return nil, err
	}
	var info PassengerInfo
	reader := bytes.NewReader([]byte(out))
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel
	err = decoder.Decode(&info)
	if err != nil {
		return nil, err
	}
	if len(info.SuperGroups) == 0 {
		level.Warn(c.logger).Log("msg", "Supergroups is empty", "instance", instance)
		return nil, nil
	}
	for _, s := range info.SuperGroups {
		var metric PassengerAppMetrics
		name := s.Group.AppRoot
		metric.Name = name
		for _, p := range s.Group.Processes {
			var processMetrics PassengerProcessMetrics
			processMetrics.RSS = p.RSS * 1024
			processMetrics.CPU = p.CPU
			processMetrics.RealMemory = p.RealMemory * 1024
			processMetrics.RequestsProcessed = p.RequestsProcessed
			startTime := p.SpawnStartTime / microsecondsPerSecond
			processMetrics.Runtime = now - startTime
			metric.Processes = append(metric.Processes, processMetrics)
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func passengerStatus(ctx context.Context, instance string, logger log.Logger) (string, error) {
	var cmds []string
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if instance != "" {
		cmds = []string{*passengerStatusPath, "--show=xml", instance}
	} else {
		cmds = []string{*passengerStatusPath, "--show=xml"}
	}
	cmd := execCommand(ctx, "sudo", cmds...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", ctx.Err()
	} else if err != nil {
		if strings.Contains(stdout.String(), "It appears that multiple") {
			return stdout.String(), nil
		}
		level.Error(logger).Log("msg", fmt.Sprintf("Error executing %s", *passengerStatusPath), "err", stderr.String(), "out", stdout.String())
		return "", err
	}
	return stdout.String(), nil
}
