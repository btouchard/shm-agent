// SPDX-License-Identifier: MIT

package agent

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/kolapsis/shm-agent/agent/config"
)

func TestAgent_ProcessJSON(t *testing.T) {
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test-app",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   "/var/log/test.log",
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "requests",
						Type: "counter",
						Match: &config.Match{
							Field:  "event",
							Equals: "request",
						},
					},
					{
						Name: "errors",
						Type: "counter",
						Match: &config.Match{
							Field: "level",
							In:    []string{"error", "fatal"},
						},
					},
					{
						Name: "total_bytes",
						Type: "sum",
						Extract: &config.Extract{
							Field: "bytes",
						},
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

	agent, err := New(Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Process lines
	lines := []string{
		`{"event": "request", "user_id": "user1", "bytes": 100}`,
		`{"event": "request", "user_id": "user2", "bytes": 200}`,
		`{"event": "request", "user_id": "user1", "bytes": 150}`,
		`{"level": "error", "message": "something went wrong"}`,
		`{"level": "info", "message": "all good"}`,
		`{"level": "fatal", "message": "critical error"}`,
	}

	for _, line := range lines {
		agent.ProcessLine(0, line)
	}

	metrics := agent.GetAggregator().Peek()

	// Check counter
	if v := metrics["requests"].(float64); v != 3 {
		t.Errorf("requests = %v, want 3", v)
	}

	// Check error counter
	if v := metrics["errors"].(float64); v != 2 {
		t.Errorf("errors = %v, want 2", v)
	}

	// Check sum
	if v := metrics["total_bytes"].(float64); v != 450 {
		t.Errorf("total_bytes = %v, want 450", v)
	}

	// Check set (unique users)
	if v := metrics["unique_users"].(int); v != 2 {
		t.Errorf("unique_users = %v, want 2", v)
	}
}

func TestAgent_ProcessRegex(t *testing.T) {
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test-app",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:    "/var/log/nginx/access.log",
				Format:  "regex",
				Pattern: `^(?P<ip>\S+) \S+ \S+ \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) [^"]*" (?P<status>\d+) (?P<bytes>\d+)`,
				Metrics: []config.Metric{
					{
						Name: "http_requests",
						Type: "counter",
					},
					{
						Name: "http_5xx",
						Type: "counter",
						Match: &config.Match{
							Field: "status",
							Regex: `^5\d{2}$`,
						},
					},
					{
						Name: "bytes_served",
						Type: "sum",
						Extract: &config.Extract{
							Field: "bytes",
						},
					},
					{
						Name: "unique_ips",
						Type: "set",
						Extract: &config.Extract{
							Field: "ip",
						},
					},
				},
			},
		},
	}

	agent, err := New(Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	lines := []string{
		`192.168.1.1 - - [15/Jan/2024:10:00:00 +0000] "GET /api/health HTTP/1.1" 200 1024`,
		`192.168.1.2 - - [15/Jan/2024:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 512`,
		`192.168.1.1 - - [15/Jan/2024:10:00:02 +0000] "GET /api/users HTTP/1.1" 500 256`,
		`192.168.1.3 - - [15/Jan/2024:10:00:03 +0000] "GET /api/error HTTP/1.1" 503 128`,
		`not a valid log line`,
	}

	for _, line := range lines {
		agent.ProcessLine(0, line)
	}

	metrics := agent.GetAggregator().Peek()

	// All valid lines should be counted
	if v := metrics["http_requests"].(float64); v != 4 {
		t.Errorf("http_requests = %v, want 4", v)
	}

	// 5xx errors
	if v := metrics["http_5xx"].(float64); v != 2 {
		t.Errorf("http_5xx = %v, want 2", v)
	}

	// Total bytes
	if v := metrics["bytes_served"].(float64); v != 1920 {
		t.Errorf("bytes_served = %v, want 1920", v)
	}

	// Unique IPs
	if v := metrics["unique_ips"].(int); v != 3 {
		t.Errorf("unique_ips = %v, want 3", v)
	}
}

func TestAgent_Gauge(t *testing.T) {
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test-app",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   "/var/log/test.log",
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "active_sessions",
						Type: "gauge",
						Extract: &config.Extract{
							Field: "sessions",
						},
					},
				},
			},
		},
	}

	agent, err := New(Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	lines := []string{
		`{"sessions": 10}`,
		`{"sessions": 15}`,
		`{"sessions": 12}`,
	}

	for _, line := range lines {
		agent.ProcessLine(0, line)
	}

	metrics := agent.GetAggregator().Peek()

	// Gauge should have last value
	if v := metrics["active_sessions"].(float64); v != 12 {
		t.Errorf("active_sessions = %v, want 12", v)
	}
}

func TestAgent_NestedFields(t *testing.T) {
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test-app",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   "/var/log/test.log",
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "response_bytes",
						Type: "sum",
						Extract: &config.Extract{
							Field: "response.bytes",
						},
					},
					{
						Name: "active_sessions",
						Type: "gauge",
						Extract: &config.Extract{
							Field: "metrics.sessions.active",
						},
					},
				},
			},
		},
	}

	agent, err := New(Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	lines := []string{
		`{"response": {"bytes": 100, "status": 200}}`,
		`{"response": {"bytes": 200, "status": 200}}`,
		`{"metrics": {"sessions": {"active": 42, "total": 100}}}`,
	}

	for _, line := range lines {
		agent.ProcessLine(0, line)
	}

	metrics := agent.GetAggregator().Peek()

	if v := metrics["response_bytes"].(float64); v != 300 {
		t.Errorf("response_bytes = %v, want 300", v)
	}

	if v := metrics["active_sessions"].(float64); v != 42 {
		t.Errorf("active_sessions = %v, want 42", v)
	}
}

func TestAgent_ProcessFile(t *testing.T) {
	// Create temp file with test data
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	content := `{"event": "request", "bytes": 100}
{"event": "request", "bytes": 200}
{"event": "error", "bytes": 50}
{"event": "request", "bytes": 150}
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test-app",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   path,
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "requests",
						Type: "counter",
						Match: &config.Match{
							Field:  "event",
							Equals: "request",
						},
					},
					{
						Name: "total_bytes",
						Type: "sum",
						Extract: &config.Extract{
							Field: "bytes",
						},
					},
				},
			},
		},
	}

	agent, err := New(Options{
		Config: cfg,
		DryRun: true,
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	count, err := agent.ProcessFile(path)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if count != 4 {
		t.Errorf("ProcessFile() count = %d, want 4", count)
	}

	metrics := agent.GetAggregator().Peek()

	if v := metrics["requests"].(float64); v != 3 {
		t.Errorf("requests = %v, want 3", v)
	}

	if v := metrics["total_bytes"].(float64); v != 500 {
		t.Errorf("total_bytes = %v, want 500", v)
	}
}

func TestAgent_MalformedLines(t *testing.T) {
	cfg := &config.Config{
		ServerURL:   "https://example.com",
		AppName:     "test-app",
		AppVersion:  "1.0.0",
		Environment: "test",
		Sources: []config.Source{
			{
				Path:   "/var/log/test.log",
				Format: "json",
				Metrics: []config.Metric{
					{
						Name: "requests",
						Type: "counter",
					},
				},
			},
		},
	}

	agent, err := New(Options{
		Config: cfg,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Mix of valid and invalid lines
	lines := []string{
		`{"event": "request"}`,
		`not json`,
		`{"event": "another"}`,
		`{"broken": }`,
		`{"event": "last"}`,
	}

	for _, line := range lines {
		agent.ProcessLine(0, line)
	}

	metrics := agent.GetAggregator().Peek()

	// Only valid JSON lines should be counted
	if v := metrics["requests"].(float64); v != 3 {
		t.Errorf("requests = %v, want 3", v)
	}
}
