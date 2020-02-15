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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
)

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
