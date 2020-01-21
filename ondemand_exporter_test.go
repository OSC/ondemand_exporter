package main

import (
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
	"testing"

	"github.com/Flaque/filet"
)

var (
	mockedExitStatus = 0
	mockedStdout     string
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelper", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
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

	fmt.Fprintf(os.Stdout, os.Getenv("STDOUT"))
	i, _ := strconv.Atoi(os.Getenv("EXIT_STATUS"))
	os.Exit(i)
}

func TestGetActivePuns(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
foo
bar`
	expPuns := []string{"foo", "bar"}
	defer func() { execCommand = exec.Command }()
	puns, _ := getActivePuns()
	if !reflect.DeepEqual(puns, expPuns) {
		t.Errorf("Expected %v, got %v", expPuns, puns)
	}
}

func TestGetApacheStatusURL_Default(t *testing.T) {
	defer func() { osHostname = os.Hostname }()
	fqdn = "foo.example.com"
	osHostname = func() (string, error) { return "foo.example.com", nil }
	ret := getApacheStatusURL()
	expected := "http://foo.example.com/server-status"
	if ret != expected {
		t.Errorf("Expected %s, got %s", expected, ret)
	}
}

func TestGetApacheStatusURL_read_ood_portal(t *testing.T) {
	defer filet.CleanUp(t)
	oodPortalYAML := `
servername: ood.example.com
port: 443`
	filet.File(t, "/tmp/ood_portal.yml", oodPortalYAML)
	oodPortalPath = "/tmp/ood_portal.yml"
	fqdn = "foo.example.com"
	ret := getApacheStatusURL()
	expected := "https://ood.example.com/server-status"
	if ret != expected {
		t.Errorf("Expected %s, got %s", expected, ret)
	}
}

func TestGetApacheMetrics(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fixture := filepath.Join(dir, "fixtures/status")
	fixtureData, err := ioutil.ReadFile(fixture)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write(fixtureData)
	}))
	defer server.Close()
	m, err := getApacheMetrics(server.URL, "ood.example.com")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if val := m.WebsocketConnections; val != 5 {
		t.Errorf("Unexpected value for WebsocketConnections, expected 5, got %v", val)
	}
	if val := m.UniqueWebsocketClients; val != 3 {
		t.Errorf("Unexpected value for UniqueWebsocketClients, expected 3, got %v", val)
	}
	if val := m.ClientConnections; val != 63 {
		t.Errorf("Unexpected value for ClientConnections, expected 63, got %v", val)
	}
	if val := m.UniqueClientConnections; val != 38 {
		t.Errorf("Unexpected value for UniqueClientConnections, expected 38, got %v", val)
	}
}

func TestGetApacheMetricsThreadMPM(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fixture := filepath.Join(dir, "fixtures/status2")
	fixtureData, err := ioutil.ReadFile(fixture)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write(fixtureData)
	}))
	defer server.Close()
	m, err := getApacheMetrics(server.URL, "ood.example.com")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if val := m.WebsocketConnections; val != 121 {
		t.Errorf("Unexpected value for WebsocketConnections, expected 121, got %v", val)
	}
	if val := m.UniqueWebsocketClients; val != 27 {
		t.Errorf("Unexpected value for UniqueWebsocketClients, expected 27, got %v", val)
	}
	if val := m.ClientConnections; val != 202 {
		t.Errorf("Unexpected value for ClientConnections, expected 202, got %v", val)
	}
	if val := m.UniqueClientConnections; val != 36 {
		t.Errorf("Unexpected value for UniqueClientConnections, expected 36, got %v", val)
	}
}
