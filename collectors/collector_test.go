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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gopkg.in/alecthomas/kingpin.v2"
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
	puns, err := getActivePuns(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if !reflect.DeepEqual(puns, expPuns) {
		t.Errorf("Expected %v, got %v", expPuns, puns)
	}
}

func TestCollector(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
foo
bar`
	defer func() { execCommand = exec.CommandContext }()
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fixture := filepath.Join(dir, "fixtures/status")
	fixtureData, err := ioutil.ReadFile(fixture)
	if err != nil {
		t.Fatalf("Error loading fixture data: %s", err.Error())
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write(fixtureData)
	}))
	defer server.Close()
	expected := `
		# HELP ondemand_active_puns Active PUNs
		# TYPE ondemand_active_puns gauge
		ondemand_active_puns 2
		# HELP ondemand_client_connections Number of client connections
		# TYPE ondemand_client_connections gauge
		ondemand_client_connections 63
		# HELP ondemand_node_apps Number of running NodeJS apps
		# TYPE ondemand_node_apps gauge
		ondemand_node_apps 0
		# HELP ondemand_pun_cpu_percent Percent CPU of all PUNs
		# TYPE ondemand_pun_cpu_percent gauge
		ondemand_pun_cpu_percent 0
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
	collector := NewCollector()
	collector.ApacheStatus = server.URL
	gatherers := setupGatherer(collector)
	if val := testutil.CollectAndCount(collector); val != 18 {
		t.Errorf("Unexpected collection count %d, expected 18", val)
	}
	if err := testutil.GatherAndCompare(gatherers, strings.NewReader(expected), "ondemand_active_puns",
		"ondemand_client_connections", "ondemand_unique_client_connections", "ondemand_unique_websocket_clients", "ondemand_websocket_connections",
		"ondemand_node_apps", "ondemand_rack_apps", "ondemand_pun_cpu_percent", "ondemand_pun_memory", "ondemand_pun_memory_percent"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func setupGatherer(collector *Collector) prometheus.Gatherer {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	gatherers := prometheus.Gatherers{registry}
	return gatherers
}
