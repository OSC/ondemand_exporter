package main

import (
	"os"
	"testing"

	"github.com/Flaque/filet"
)

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

