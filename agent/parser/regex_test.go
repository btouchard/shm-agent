// SPDX-License-Identifier: MIT

package parser

import (
	"reflect"
	"testing"
)

func TestRegexParser_Parse(t *testing.T) {
	pattern := `^(?P<ip>\S+) \S+ \S+ \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) [^"]*" (?P<status>\d+) (?P<bytes>\d+)`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	line := `192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api/health HTTP/1.1" 200 1234`

	result := p.Parse(line)
	if result == nil {
		t.Fatal("Parse() = nil, want non-nil")
	}

	expected := map[string]interface{}{
		"ip":     "192.168.1.1",
		"time":   "15/Jan/2024:10:30:00 +0000",
		"method": "GET",
		"path":   "/api/health",
		"status": "200",
		"bytes":  "1234",
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("result[%q] = %v, want %v", k, result[k], v)
		}
	}
}

func TestRegexParser_NoMatch(t *testing.T) {
	pattern := `^(?P<ip>\d+\.\d+\.\d+\.\d+)`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	line := "not an ip address"
	result := p.Parse(line)
	if result != nil {
		t.Errorf("Parse() = %v, want nil", result)
	}
}

func TestRegexParser_InvalidPattern(t *testing.T) {
	_, err := NewRegexParser("[invalid")
	if err == nil {
		t.Error("NewRegexParser() should return error for invalid pattern")
	}
}

func TestRegexParser_NoNamedGroups(t *testing.T) {
	pattern := `^\d+\.\d+\.\d+\.\d+`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	line := "192.168.1.1 some text"
	result := p.Parse(line)
	// No named groups, so result should be nil
	if result != nil {
		t.Errorf("Parse() = %v, want nil (no named groups)", result)
	}
}

func TestRegexParser_GroupNames(t *testing.T) {
	pattern := `^(?P<ip>\S+) (?P<method>\S+) (?P<path>\S+)`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	names := p.GroupNames()
	expected := []string{"ip", "method", "path"}

	if !reflect.DeepEqual(names, expected) {
		t.Errorf("GroupNames() = %v, want %v", names, expected)
	}
}

func TestRegexParser_Pattern(t *testing.T) {
	pattern := `^(?P<level>\w+):`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	if p.Pattern() != pattern {
		t.Errorf("Pattern() = %q, want %q", p.Pattern(), pattern)
	}
}

func TestRegexParser_NginxCombinedLog(t *testing.T) {
	pattern := `^(?P<ip>\S+) \S+ \S+ \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) (?P<protocol>[^"]*)" (?P<status>\d+) (?P<bytes>\d+) "(?P<referer>[^"]*)" "(?P<useragent>[^"]*)"`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	lines := []struct {
		line     string
		expected map[string]interface{}
	}{
		{
			line: `93.180.71.3 - - [17/May/2015:08:05:32 +0000] "GET /downloads/product_1 HTTP/1.1" 304 0 "-" "Debian APT-HTTP/1.3 (0.8.16~exp12ubuntu10.21)"`,
			expected: map[string]interface{}{
				"ip":        "93.180.71.3",
				"method":    "GET",
				"path":      "/downloads/product_1",
				"protocol":  "HTTP/1.1",
				"status":    "304",
				"bytes":     "0",
				"referer":   "-",
				"useragent": "Debian APT-HTTP/1.3 (0.8.16~exp12ubuntu10.21)",
			},
		},
		{
			line: `10.0.0.1 - admin [15/Jan/2024:12:00:00 +0000] "POST /api/login HTTP/2.0" 200 512 "https://example.com" "Mozilla/5.0"`,
			expected: map[string]interface{}{
				"ip":        "10.0.0.1",
				"method":    "POST",
				"path":      "/api/login",
				"protocol":  "HTTP/2.0",
				"status":    "200",
				"bytes":     "512",
				"referer":   "https://example.com",
				"useragent": "Mozilla/5.0",
			},
		},
	}

	for _, tt := range lines {
		result := p.Parse(tt.line)
		if result == nil {
			t.Errorf("Parse(%q) = nil, want non-nil", tt.line)
			continue
		}
		for k, v := range tt.expected {
			if result[k] != v {
				t.Errorf("result[%q] = %v, want %v", k, result[k], v)
			}
		}
	}
}

func TestRegexParser_SyslogAuth(t *testing.T) {
	pattern := `^(?P<month>\w+)\s+(?P<day>\d+)\s+(?P<time>\S+)\s+(?P<host>\S+)\s+(?P<service>\S+)\[(?P<pid>\d+)\]:\s+(?P<message>.*)$`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	lines := []struct {
		line     string
		expected map[string]interface{}
	}{
		{
			line: `Jan 15 10:30:00 server sshd[12345]: Failed password for invalid user admin from 192.168.1.100 port 54321 ssh2`,
			expected: map[string]interface{}{
				"month":   "Jan",
				"day":     "15",
				"time":    "10:30:00",
				"host":    "server",
				"service": "sshd",
				"pid":     "12345",
				"message": "Failed password for invalid user admin from 192.168.1.100 port 54321 ssh2",
			},
		},
		{
			line: `Jan 15 10:30:05 server sshd[12346]: Accepted publickey for ubuntu from 10.0.0.1 port 22 ssh2`,
			expected: map[string]interface{}{
				"month":   "Jan",
				"day":     "15",
				"time":    "10:30:05",
				"host":    "server",
				"service": "sshd",
				"pid":     "12346",
				"message": "Accepted publickey for ubuntu from 10.0.0.1 port 22 ssh2",
			},
		},
	}

	for _, tt := range lines {
		result := p.Parse(tt.line)
		if result == nil {
			t.Errorf("Parse(%q) = nil, want non-nil", tt.line)
			continue
		}
		for k, v := range tt.expected {
			if result[k] != v {
				t.Errorf("result[%q] = %v, want %v", k, result[k], v)
			}
		}
	}
}

func TestRegexParser_EmptyLine(t *testing.T) {
	pattern := `^(?P<level>\w+)`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	result := p.Parse("")
	if result != nil {
		t.Errorf("Parse('') = %v, want nil", result)
	}
}

func TestRegexParser_PartialMatch(t *testing.T) {
	// Pattern that might partially match
	pattern := `^(?P<level>INFO|ERROR|WARN):\s+(?P<message>.*)`
	p, err := NewRegexParser(pattern)
	if err != nil {
		t.Fatalf("NewRegexParser() error = %v", err)
	}

	tests := []struct {
		line    string
		wantNil bool
		level   string
		message string
	}{
		{"INFO: Application started", false, "INFO", "Application started"},
		{"ERROR: Something went wrong", false, "ERROR", "Something went wrong"},
		{"DEBUG: This won't match", true, "", ""},
		{"INFO Application started", true, "", ""}, // Missing colon
	}

	for _, tt := range tests {
		result := p.Parse(tt.line)
		if tt.wantNil {
			if result != nil {
				t.Errorf("Parse(%q) = %v, want nil", tt.line, result)
			}
			continue
		}
		if result == nil {
			t.Errorf("Parse(%q) = nil, want non-nil", tt.line)
			continue
		}
		if result["level"] != tt.level {
			t.Errorf("Parse(%q)[level] = %v, want %v", tt.line, result["level"], tt.level)
		}
		if result["message"] != tt.message {
			t.Errorf("Parse(%q)[message] = %v, want %v", tt.line, result["message"], tt.message)
		}
	}
}
