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
