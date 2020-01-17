package main

import (
	"fmt"
	"github.com/Flaque/filet"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"testing"
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
