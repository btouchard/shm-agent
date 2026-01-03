// SPDX-License-Identifier: MIT

package config

import (
	"testing"
	"time"
)

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"
environment: production
interval: 30s

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: counter
        match:
          field: event
          equals: "request"
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerURL != "https://shm.example.com" {
		t.Errorf("ServerURL = %q, want %q", cfg.ServerURL, "https://shm.example.com")
	}

	if cfg.AppName != "my-app" {
		t.Errorf("AppName = %q, want %q", cfg.AppName, "my-app")
	}

	if cfg.Interval != 30*time.Second {
		t.Errorf("Interval = %v, want %v", cfg.Interval, 30*time.Second)
	}

	if len(cfg.Sources) != 1 {
		t.Errorf("len(Sources) = %d, want 1", len(cfg.Sources))
	}
}

func TestParse_Defaults(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: counter
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.IdentityFile != "./shm_identity.json" {
		t.Errorf("IdentityFile = %q, want %q", cfg.IdentityFile, "./shm_identity.json")
	}

	if cfg.Interval != 60*time.Second {
		t.Errorf("Interval = %v, want %v", cfg.Interval, 60*time.Second)
	}

	if cfg.Environment != "production" {
		t.Errorf("Environment = %q, want %q", cfg.Environment, "production")
	}
}

func TestParse_MissingServerURL(t *testing.T) {
	yaml := `
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: counter
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing server_url")
	}
}

func TestParse_MissingAppName(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: counter
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing app_name")
	}
}

func TestParse_InvalidFormat(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: xml
    metrics:
      - name: requests
        type: counter
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestParse_RegexWithoutPattern(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: regex
    metrics:
      - name: requests
        type: counter
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for regex format without pattern")
	}
}

func TestParse_InvalidRegexPattern(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: regex
    pattern: "[invalid"
    metrics:
      - name: requests
        type: counter
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

func TestParse_InvalidMetricType(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: histogram
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid metric type")
	}
}

func TestParse_SumWithoutExtract(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: total_bytes
        type: sum
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for sum without extract")
	}
}

func TestParse_MatchMultipleConditions(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: counter
        match:
          field: event
          equals: "request"
          contains: "request"
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for multiple match conditions")
	}
}

func TestParse_MatchNoCondition(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: counter
        match:
          field: event
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for match without condition")
	}
}

func TestParse_ComplexConfig(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
identity_file: /var/lib/shm-agent/identity.json
app_name: my-app
app_version: "1.0.0"
environment: production
interval: 60s

sources:
  - path: /var/log/myapp/app.log
    format: json
    metrics:
      - name: requests_count
        type: counter
        match:
          field: event
          equals: "request_processed"

      - name: errors_count
        type: counter
        match:
          field: level
          in: ["error", "fatal"]

      - name: active_sessions
        type: gauge
        extract:
          field: metrics.active_sessions

      - name: unique_users
        type: set
        extract:
          field: user_id

      - name: total_bytes
        type: sum
        extract:
          field: response.bytes

  - path: /var/log/nginx/access.log
    format: regex
    pattern: '^(?P<ip>\S+) \S+ \S+ \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) [^"]*" (?P<status>\d+) (?P<bytes>\d+)'
    metrics:
      - name: http_requests
        type: counter

      - name: http_5xx
        type: counter
        match:
          field: status
          regex: "^5\\d{2}$"

      - name: bytes_served
        type: sum
        extract:
          field: bytes

      - name: unique_ips
        type: set
        extract:
          field: ip
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Sources) != 2 {
		t.Errorf("len(Sources) = %d, want 2", len(cfg.Sources))
	}

	// Check JSON source
	jsonSource := cfg.Sources[0]
	if jsonSource.Format != "json" {
		t.Errorf("Sources[0].Format = %q, want %q", jsonSource.Format, "json")
	}
	if len(jsonSource.Metrics) != 5 {
		t.Errorf("len(Sources[0].Metrics) = %d, want 5", len(jsonSource.Metrics))
	}

	// Check regex source
	regexSource := cfg.Sources[1]
	if regexSource.Format != "regex" {
		t.Errorf("Sources[1].Format = %q, want %q", regexSource.Format, "regex")
	}
	if regexSource.Pattern == "" {
		t.Error("Sources[1].Pattern should not be empty")
	}
	if len(regexSource.Metrics) != 4 {
		t.Errorf("len(Sources[1].Metrics) = %d, want 4", len(regexSource.Metrics))
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
  invalid indentation
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_NoSources(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"
sources: []
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty sources")
	}
}

func TestParse_IntervalTooShort(t *testing.T) {
	yaml := `
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"
interval: 500ms

sources:
  - path: /var/log/app.log
    format: json
    metrics:
      - name: requests
        type: counter
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for interval too short")
	}
}
