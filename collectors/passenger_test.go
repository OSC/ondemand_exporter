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
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/prometheus/common/promslog"
)

func TestGetInstances(t *testing.T) {
	logger := promslog.NewNopLogger()
	passengerStatusExec = func(ctx context.Context, instance string, logger *slog.Logger) (string, error) {
		return readFixture("passenger-status.out"), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collector := NewPassengerCollector(logger)
	m, err := collector.getInstances([]string{"32666", "20821"}, ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(m) != 2 {
		t.Errorf("Unexpected count of instances: %d", len(m))
	}
}

func TestGetInstancesOne(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	procFS = filepath.Join(dir, "../fixtures/proc")
	passengerStatusExec = func(ctx context.Context, instance string, logger *slog.Logger) (string, error) {
		return readFixture("passenger-status-57564.out"), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger := promslog.NewNopLogger()
	collector := NewPassengerCollector(logger)
	m, err := collector.getInstances([]string{"32666"}, ctx)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(m) != 1 {
		t.Errorf("Unexpected count of instances: %d", len(m))
	}
}
