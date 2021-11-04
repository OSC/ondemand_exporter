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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	mockedExitStatus = 0
	mockedStdout     string
	ctx, _           = context.WithTimeout(context.Background(), 5*time.Second)
)

func fakeExecCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelper", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	es := strconv.Itoa(mockedExitStatus)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1",
		"STDOUT=" + mockedStdout,
		"EXIT_STATUS=" + es}
	return cmd
}

func TestExecCommandHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	//nolint:staticcheck
	fmt.Fprintf(os.Stdout, os.Getenv("STDOUT"))
	i, _ := strconv.Atoi(os.Getenv("EXIT_STATUS"))
	os.Exit(i)
}

func TestGetActivePuns(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	execCommand = fakeExecCommand
	mockedStdout = `
foo
bar`
	expPuns := []string{"foo", "bar"}
	defer func() { execCommand = exec.CommandContext }()
	puns, _, err := getActivePuns(ctx, log.NewNopLogger())
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if !reflect.DeepEqual(puns, expPuns) {
		t.Errorf("Expected %v, got %v", expPuns, puns)
	}
}

func TestCollector(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	execCommand = fakeExecCommand
	mockedStdout = `
foo
bar`
	defer func() { execCommand = exec.CommandContext }()
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fixture := filepath.Join(dir, "../fixtures/status")
	fixtureData, err := ioutil.ReadFile(fixture)
	if err != nil {
		t.Fatalf("Error loading fixture data: %s", err.Error())
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write(fixtureData)
	}))
	defer server.Close()
	apacheStatusURL = &server.URL
	tmpDir, err := ioutil.TempDir(os.TempDir(), "passenger")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	passengerStatus := tmpDir + "/ondemand-passenger-status"
	if err := ioutil.WriteFile(passengerStatus, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	passengerStatusPath = &passengerStatus
	passengerStatusExec = func(ctx context.Context, instance string, logger log.Logger) (string, error) {
		return readFixture("passenger-status.out"), nil
	}
	passengerStatusExecInstance = func(ctx context.Context, instance string, logger log.Logger) (string, error) {
		return readFixture(fmt.Sprintf("passenger-status-%s.out", instance)), nil
	}
	timeNow = func() time.Time {
		mockNow, _ := time.Parse("01/02/2006", "04/17/2020")
		return mockNow
	}
	cores = func() int {
		return 1
	}
	expected := `
		# HELP ondemand_active_puns Active PUNs
		# TYPE ondemand_active_puns gauge
		ondemand_active_puns 2
		# HELP ondemand_client_connections Number of client connections
		# TYPE ondemand_client_connections gauge
		ondemand_client_connections 63
		# HELP ondemand_exporter_collect_error Indicates the collector had an error
		# TYPE ondemand_exporter_collect_error gauge
		ondemand_exporter_collect_error{collector="apache"} 0
		ondemand_exporter_collect_error{collector="passenger"} 0
		ondemand_exporter_collect_error{collector="process"} 0
		ondemand_exporter_collect_error{collector="puns"} 0
		# HELP ondemand_node_apps Number of running NodeJS apps
		# TYPE ondemand_node_apps gauge
		ondemand_node_apps 0
		# HELP ondemand_passenger_instances Number of Passenger instances
		# TYPE ondemand_passenger_instances gauge
		ondemand_passenger_instances 2
		# HELP ondemand_passenger_app_average_runtime_seconds Average runtime in seconds of passenger apps
		# TYPE ondemand_passenger_app_average_runtime_seconds gauge
		ondemand_passenger_app_average_runtime_seconds{app="/var/www/ood/apps/sys/dashboard"} 36369
		ondemand_passenger_app_average_runtime_seconds{app="/var/www/ood/apps/sys/files"} 36799
		# HELP ondemand_passenger_app_count Count of passenger instances of an app
		# TYPE ondemand_passenger_app_count gauge
		ondemand_passenger_app_count{app="/var/www/ood/apps/sys/dashboard"} 2
		ondemand_passenger_app_count{app="/var/www/ood/apps/sys/files"} 1
		# HELP ondemand_passenger_app_cpu_percent CPU percent of passenger apps
		# TYPE ondemand_passenger_app_cpu_percent gauge
		ondemand_passenger_app_cpu_percent{app="/var/www/ood/apps/sys/dashboard"} 2
		ondemand_passenger_app_cpu_percent{app="/var/www/ood/apps/sys/files"} 0
		# HELP ondemand_passenger_app_processes Process count of an app
		# TYPE ondemand_passenger_app_processes gauge
		ondemand_passenger_app_processes{app="/var/www/ood/apps/sys/dashboard"} 2
		ondemand_passenger_app_processes{app="/var/www/ood/apps/sys/files"} 1
		# HELP ondemand_passenger_app_real_memory_bytes Real memory of passenger apps
		# TYPE ondemand_passenger_app_real_memory_bytes gauge
		ondemand_passenger_app_real_memory_bytes{app="/var/www/ood/apps/sys/dashboard"} 187949056
		ondemand_passenger_app_real_memory_bytes{app="/var/www/ood/apps/sys/files"} 42954752
		# HELP ondemand_passenger_app_requests_total Requests made to passenger apps
		# TYPE ondemand_passenger_app_requests_total counter
		ondemand_passenger_app_requests_total{app="/var/www/ood/apps/sys/dashboard"} 342
		ondemand_passenger_app_requests_total{app="/var/www/ood/apps/sys/files"} 10
		# HELP ondemand_passenger_app_rss_bytes RSS of passenger apps
		# TYPE ondemand_passenger_app_rss_bytes gauge
		ondemand_passenger_app_rss_bytes{app="/var/www/ood/apps/sys/dashboard"} 202727424
		ondemand_passenger_app_rss_bytes{app="/var/www/ood/apps/sys/files"} 56840192
		# HELP ondemand_pun_cpu_time CPU time of all PUNs
		# TYPE ondemand_pun_cpu_time gauge
		ondemand_pun_cpu_time 0
		# HELP ondemand_pun_memory Memory used by all PUNs
		# TYPE ondemand_pun_memory gauge
		ondemand_pun_memory{type="rss"} 0
		ondemand_pun_memory{type="vms"} 0
		# HELP ondemand_pun_memory_percent Percent memory of all PUNs
		# TYPE ondemand_pun_memory_percent gauge
		ondemand_pun_memory_percent 0
		# HELP ondemand_rack_apps Number of running Rack apps
		# TYPE ondemand_rack_apps gauge
		ondemand_rack_apps 0
		# HELP ondemand_unique_client_connections Unique client connections
		# TYPE ondemand_unique_client_connections gauge
		ondemand_unique_client_connections 38
		# HELP ondemand_unique_websocket_clients Unique websocket connections
		# TYPE ondemand_unique_websocket_clients gauge
		ondemand_unique_websocket_clients 3
		# HELP ondemand_websocket_connections Number of websocket connections
		# TYPE ondemand_websocket_connections gauge
		ondemand_websocket_connections 5
	`
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	//logger := log.NewNopLogger()
	collector := NewCollector(logger)
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 37 {
		t.Errorf("Unexpected collection count %d, expected 37", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "ondemand_active_puns", "ondemand_exporter_collect_error",
		"ondemand_client_connections", "ondemand_unique_client_connections", "ondemand_unique_websocket_clients", "ondemand_websocket_connections",
		"ondemand_node_apps", "ondemand_rack_apps", "ondemand_pun_cpu_time", "ondemand_pun_memory", "ondemand_pun_memory_percent",
		"ondemand_passenger_instances", "ondemand_passenger_app_count", "ondemand_passenger_app_processes",
		"ondemand_passenger_app_rss_bytes", "ondemand_passenger_app_real_memory_bytes", "ondemand_passenger_app_cpu_percent",
		"ondemand_passenger_app_requests_total", "ondemand_passenger_app_average_runtime_seconds"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestCollectorNoPassengerStatus(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	execCommand = fakeExecCommand
	mockedStdout = `
foo
bar`
	defer func() { execCommand = exec.CommandContext }()
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fixture := filepath.Join(dir, "../fixtures/status")
	fixtureData, err := ioutil.ReadFile(fixture)
	if err != nil {
		t.Fatalf("Error loading fixture data: %s", err.Error())
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write(fixtureData)
	}))
	defer server.Close()
	apacheStatusURL = &server.URL
	tmpDir, err := ioutil.TempDir(os.TempDir(), "passenger")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	passengerStatus := tmpDir + "/ondemand-passenger-status"
	if err := ioutil.WriteFile(passengerStatus, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	passengerStatusPath = &passengerStatus
	passengerStatusExec = func(ctx context.Context, instance string, logger log.Logger) (string, error) {
		return readFixture("passenger-status-none.out"), nil
	}
	timeNow = func() time.Time {
		mockNow, _ := time.Parse("01/02/2006", "04/17/2020")
		return mockNow
	}
	cores = func() int {
		return 1
	}
	expected := `
		# HELP ondemand_exporter_collect_error Indicates the collector had an error
		# TYPE ondemand_exporter_collect_error gauge
		ondemand_exporter_collect_error{collector="apache"} 0
		ondemand_exporter_collect_error{collector="passenger"} 0
		ondemand_exporter_collect_error{collector="process"} 0
		ondemand_exporter_collect_error{collector="puns"} 0
		# HELP ondemand_passenger_instances Number of Passenger instances
		# TYPE ondemand_passenger_instances gauge
		ondemand_passenger_instances 0
	`
	collector := NewCollector(log.NewNopLogger())
	gatherers := setupGatherer(collector)
	if val, err := testutil.GatherAndCount(gatherers); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if val != 23 {
		t.Errorf("Unexpected collection count %d, expected 23", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "ondemand_exporter_collect_error",
		"ondemand_passenger_instances", "ondemand_passenger_app_count", "ondemand_passenger_app_processes",
		"ondemand_passenger_app_rss_bytes", "ondemand_passenger_app_real_memory_bytes", "ondemand_passenger_app_cpu_percent",
		"ondemand_passenger_app_requests_total", "ondemand_passenger_app_average_runtime_seconds"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func setupGatherer(collector *Collector) prometheus.Gatherer {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	gatherers := prometheus.Gatherers{registry}
	return gatherers
}

func readFixture(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fixture := filepath.Join(dir, fmt.Sprintf("../fixtures/%s", name))
	fixtureData, _ := ioutil.ReadFile(fixture)
	return string(fixtureData)
}
