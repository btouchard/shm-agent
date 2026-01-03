// SPDX-License-Identifier: MIT

package agent_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kolapsis/shm-agent/agent"
	"github.com/kolapsis/shm-agent/agent/config"
)

// getTestdataPath returns the path to the testdata directory.
func getTestdataPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "testdata")
}

func TestIntegration_NginxLogs(t *testing.T) {
	testdataPath := getTestdataPath()
	configPath := filepath.Join(testdataPath, "configs", "nginx.yaml")
	logPath := filepath.Join(testdataPath, "logs", "nginx_access.log")

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config error = %v", err)
	}

	// Override path for test
	cfg.Sources[0].Path = logPath

	ag, err := agent.New(agent.Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New agent error = %v", err)
	}

	count, err := ag.ProcessFile(logPath)
	if err != nil {
		t.Fatalf("ProcessFile error = %v", err)
	}

	if count != 25 {
		t.Errorf("ProcessFile count = %d, want 25", count)
	}

	metrics := ag.GetAggregator().Peek()

	// Check total requests
	if v := metrics["http_requests"].(float64); v != 25 {
		t.Errorf("http_requests = %v, want 25", v)
	}

	// Check 2xx responses (200, 201, 204, 304 = success)
	// 200: 10, 201: 1, 204: 1, 304: 7 = 19 total 2xx
	if v := metrics["http_2xx"].(float64); v != 12 {
		t.Errorf("http_2xx = %v, want 12 (200s and 201s and 204s)", v)
	}

	// Check 4xx responses (401, 403, 404)
	if v := metrics["http_4xx"].(float64); v != 4 {
		t.Errorf("http_4xx = %v, want 4", v)
	}

	// Check 5xx responses (500, 502, 503)
	if v := metrics["http_5xx"].(float64); v != 3 {
		t.Errorf("http_5xx = %v, want 3", v)
	}

	// Check bytes served (sum of all bytes)
	if v := metrics["bytes_served"].(float64); v <= 0 {
		t.Errorf("bytes_served = %v, should be > 0", v)
	}

	// Check unique IPs
	if v := metrics["unique_ips"].(int); v < 5 {
		t.Errorf("unique_ips = %v, should be >= 5", v)
	}
}

func TestIntegration_TraefikLogs(t *testing.T) {
	testdataPath := getTestdataPath()
	configPath := filepath.Join(testdataPath, "configs", "traefik.yaml")
	logPath := filepath.Join(testdataPath, "logs", "traefik_access.json")

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config error = %v", err)
	}

	cfg.Sources[0].Path = logPath

	ag, err := agent.New(agent.Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New agent error = %v", err)
	}

	count, err := ag.ProcessFile(logPath)
	if err != nil {
		t.Fatalf("ProcessFile error = %v", err)
	}

	if count != 15 {
		t.Errorf("ProcessFile count = %d, want 15", count)
	}

	metrics := ag.GetAggregator().Peek()

	// Check total requests
	if v := metrics["http_requests"].(float64); v != 15 {
		t.Errorf("http_requests = %v, want 15", v)
	}

	// Check successful responses (2xx)
	// 200: 7, 201: 1, 204: 1 = 9 success
	if v := metrics["http_success"].(float64); v != 9 {
		t.Errorf("http_success = %v, want 9", v)
	}

	// Check error responses (4xx + 5xx)
	// 401: 1, 403: 1, 404: 1, 500: 1, 502: 1, 503: 1 = 6 errors
	if v := metrics["http_errors"].(float64); v != 6 {
		t.Errorf("http_errors = %v, want 6", v)
	}

	// Check unique clients
	if v := metrics["unique_clients"].(int); v != 10 {
		t.Errorf("unique_clients = %v, want 10", v)
	}

	// Check total duration (should be sum of all durations)
	if v := metrics["total_duration_ns"].(float64); v <= 0 {
		t.Errorf("total_duration_ns = %v, should be > 0", v)
	}
}

func TestIntegration_AppJSONLogs(t *testing.T) {
	testdataPath := getTestdataPath()
	configPath := filepath.Join(testdataPath, "configs", "app_json.yaml")
	logPath := filepath.Join(testdataPath, "logs", "app.log")

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config error = %v", err)
	}

	cfg.Sources[0].Path = logPath

	ag, err := agent.New(agent.Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New agent error = %v", err)
	}

	count, err := ag.ProcessFile(logPath)
	if err != nil {
		t.Fatalf("ProcessFile error = %v", err)
	}

	if count != 20 {
		t.Errorf("ProcessFile count = %d, want 20", count)
	}

	metrics := ag.GetAggregator().Peek()

	// Check requests processed
	if v := metrics["requests_processed"].(float64); v != 11 {
		t.Errorf("requests_processed = %v, want 11", v)
	}

	// Check errors (error + fatal levels)
	if v := metrics["errors_count"].(float64); v != 4 {
		t.Errorf("errors_count = %v, want 4", v)
	}

	// Check warnings
	if v := metrics["warnings_count"].(float64); v != 3 {
		t.Errorf("warnings_count = %v, want 3", v)
	}

	// Check unique users
	if v := metrics["unique_users"].(int); v >= 5 {
		// Good - we have multiple unique users
	} else {
		t.Errorf("unique_users = %v, should be >= 5", v)
	}

	// Check total duration (sum of all duration_ms)
	if v := metrics["total_duration_ms"].(float64); v <= 0 {
		t.Errorf("total_duration_ms = %v, should be > 0", v)
	}

	// Check database errors (events containing "database")
	if v := metrics["database_errors"].(float64); v != 2 {
		t.Errorf("database_errors = %v, want 2", v)
	}
}

func TestIntegration_MalformedLogs(t *testing.T) {
	testdataPath := getTestdataPath()
	logPath := filepath.Join(testdataPath, "logs", "malformed.log")

	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   logPath,
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "valid_events",
						Type: "counter",
						Match: &config.Match{
							Field:    "event",
							Contains: "valid",
						},
					},
				},
			},
		},
	}

	ag, err := agent.New(agent.Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New agent error = %v", err)
	}

	count, err := ag.ProcessFile(logPath)
	if err != nil {
		t.Fatalf("ProcessFile error = %v", err)
	}

	// Should process all lines without crashing
	if count != 11 {
		t.Errorf("ProcessFile count = %d, want 11", count)
	}

	metrics := ag.GetAggregator().Peek()

	// Only valid JSON with "event" containing "valid" should be counted
	// Line 1: valid_event, Line 3: another_valid, Line 5: valid_error,
	// Line 8: valid_warning, Line 10: last_valid = 5 valid events
	if v := metrics["valid_events"].(float64); v != 5 {
		t.Errorf("valid_events = %v, want 5", v)
	}
}

func TestIntegration_InvalidConfig(t *testing.T) {
	testdataPath := getTestdataPath()
	configPath := filepath.Join(testdataPath, "configs", "invalid.yaml")

	_, err := config.Load(configPath)
	if err == nil {
		t.Error("Load should return error for invalid config")
	}
}

func TestIntegration_CounterVsSet(t *testing.T) {
	// Test that counter counts all occurrences while set counts unique values
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   "/tmp/test.log",
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "total_requests",
						Type: "counter",
					},
					{
						Name: "unique_users",
						Type: "set",
						Extract: &config.Extract{
							Field: "user_id",
						},
					},
				},
			},
		},
	}

	ag, err := agent.New(agent.Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New agent error = %v", err)
	}

	// Same user multiple times
	lines := []string{
		`{"user_id": "user1"}`,
		`{"user_id": "user1"}`,
		`{"user_id": "user2"}`,
		`{"user_id": "user1"}`,
		`{"user_id": "user3"}`,
	}

	for _, line := range lines {
		ag.ProcessLine(0, line)
	}

	metrics := ag.GetAggregator().Peek()

	// Counter should count all
	if v := metrics["total_requests"].(float64); v != 5 {
		t.Errorf("total_requests = %v, want 5", v)
	}

	// Set should count unique
	if v := metrics["unique_users"].(int); v != 3 {
		t.Errorf("unique_users = %v, want 3", v)
	}
}

func TestIntegration_SumVsGauge(t *testing.T) {
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   "/tmp/test.log",
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "total_bytes",
						Type: "sum",
						Extract: &config.Extract{
							Field: "bytes",
						},
					},
					{
						Name: "current_connections",
						Type: "gauge",
						Extract: &config.Extract{
							Field: "connections",
						},
					},
				},
			},
		},
	}

	ag, err := agent.New(agent.Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New agent error = %v", err)
	}

	lines := []string{
		`{"bytes": 100, "connections": 5}`,
		`{"bytes": 200, "connections": 8}`,
		`{"bytes": 150, "connections": 6}`,
	}

	for _, line := range lines {
		ag.ProcessLine(0, line)
	}

	metrics := ag.GetAggregator().Peek()

	// Sum should add all values
	if v := metrics["total_bytes"].(float64); v != 450 {
		t.Errorf("total_bytes = %v, want 450", v)
	}

	// Gauge should have last value
	if v := metrics["current_connections"].(float64); v != 6 {
		t.Errorf("current_connections = %v, want 6", v)
	}
}

func TestIntegration_SnapshotReset(t *testing.T) {
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   "/tmp/test.log",
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "counter",
						Type: "counter",
					},
					{
						Name: "gauge",
						Type: "gauge",
						Extract: &config.Extract{
							Field: "value",
						},
					},
				},
			},
		},
	}

	ag, err := agent.New(agent.Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New agent error = %v", err)
	}

	// Process some lines
	ag.ProcessLine(0, `{"value": 10}`)
	ag.ProcessLine(0, `{"value": 20}`)

	// First snapshot
	agg := ag.GetAggregator()
	metrics1 := agg.Snapshot()

	if v := metrics1["counter"].(float64); v != 2 {
		t.Errorf("first snapshot counter = %v, want 2", v)
	}
	if v := metrics1["gauge"].(float64); v != 20 {
		t.Errorf("first snapshot gauge = %v, want 20", v)
	}

	// After snapshot, counter should reset but gauge should not
	metrics2 := agg.Peek()

	if v := metrics2["counter"].(float64); v != 0 {
		t.Errorf("after snapshot counter = %v, want 0 (reset)", v)
	}
	if v := metrics2["gauge"].(float64); v != 20 {
		t.Errorf("after snapshot gauge = %v, want 20 (no reset)", v)
	}
}
