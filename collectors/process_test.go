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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-kit/kit/log"
)

func TestGetProcessMetrics(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	procFS = filepath.Join(dir, "../fixtures/proc")
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	//logger := log.NewNopLogger()
	m, err := getProcessMetrics([]string{"32666", "20821"}, logger)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if val := m.RackApps; val != 4 {
		t.Errorf("Unexpected value for RackApps, expected 4, got %v", val)
	}
	if val := m.NodeApps; val != 1 {
		t.Errorf("Unexpected value for NodeApps, expected 1, got %v", val)
	}
	if val := fmt.Sprintf("%.2f", m.PunCpuTime); val != "9.95" {
		t.Errorf("Unexpected value for PunCpuTime, expected 9.95, got %v", val)
	}
	if val := m.PunMemoryRSS; val != 338735104 {
		t.Errorf("Unexpected value for PunMemoryRSS, expected 338735104, got %v", val)
	}
	if val := m.PunMemoryVMS; val != 4490481664 {
		t.Errorf("Unexpected value for PunMemoryVMS, expected 4490481664, got %v", val)
	}
	if val := fmt.Sprintf("%.2f", m.PunMemoryPercent); val != "2.04" {
		t.Errorf("Unexpected value for PunMemoryPercent, expected 2.04, got %v", val)
	}
}
