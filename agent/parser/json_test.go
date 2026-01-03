// SPDX-License-Identifier: MIT

package parser

import (
	"testing"
)

func TestJSONParser_Parse(t *testing.T) {
	p := NewJSONParser()

	tests := []struct {
		name    string
		line    string
		wantNil bool
		check   func(map[string]interface{}) bool
	}{
		{
			name: "simple object",
			line: `{"event": "request", "status": 200}`,
			check: func(data map[string]interface{}) bool {
				return data["event"] == "request" && data["status"] == float64(200)
			},
		},
		{
			name: "nested object",
			line: `{"metrics": {"cpu": 45.5, "memory": 1024}}`,
			check: func(data map[string]interface{}) bool {
				metrics, ok := data["metrics"].(map[string]interface{})
				return ok && metrics["cpu"] == 45.5
			},
		},
		{
			name: "with array",
			line: `{"tags": ["web", "api"], "count": 3}`,
			check: func(data map[string]interface{}) bool {
				tags, ok := data["tags"].([]interface{})
				return ok && len(tags) == 2
			},
		},
		{
			name:    "invalid json",
			line:    `{"event": "request"`,
			wantNil: true,
		},
		{
			name:    "empty line",
			line:    "",
			wantNil: true,
		},
		{
			name:    "plain text",
			line:    "This is not JSON",
			wantNil: true,
		},
		{
			name:    "json array (not object)",
			line:    `[1, 2, 3]`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.line)
			if tt.wantNil {
				if result != nil {
					t.Errorf("Parse() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Fatal("Parse() = nil, want non-nil")
			}
			if tt.check != nil && !tt.check(result) {
				t.Errorf("Parse() check failed for %v", result)
			}
		})
	}
}

func TestGetField(t *testing.T) {
	data := map[string]interface{}{
		"level": "info",
		"count": float64(42),
		"metrics": map[string]interface{}{
			"cpu": 45.5,
			"nested": map[string]interface{}{
				"deep": "value",
			},
		},
	}

	tests := []struct {
		field string
		want  interface{}
		ok    bool
	}{
		{"level", "info", true},
		{"count", float64(42), true},
		{"metrics.cpu", 45.5, true},
		{"metrics.nested.deep", "value", true},
		{"missing", nil, false},
		{"metrics.missing", nil, false},
		{"metrics.cpu.invalid", nil, false},
		{"", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, ok := GetField(data, tt.field)
			if ok != tt.ok {
				t.Errorf("GetField(%q) ok = %v, want %v", tt.field, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("GetField(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestGetField_NilData(t *testing.T) {
	_, ok := GetField(nil, "field")
	if ok {
		t.Error("GetField(nil, ...) should return false")
	}
}

func TestGetFieldString(t *testing.T) {
	data := map[string]interface{}{
		"string_val": "hello",
		"float_val":  float64(42.5),
		"int_val":    float64(100), // JSON numbers are float64
		"bool_val":   true,
		"nested": map[string]interface{}{
			"value": "nested_string",
		},
	}

	tests := []struct {
		field string
		want  string
		ok    bool
	}{
		{"string_val", "hello", true},
		{"float_val", "42.5", true},
		{"int_val", "100", true},
		{"bool_val", "true", true},
		{"nested.value", "nested_string", true},
		{"missing", "", false},
		{"nested", "", false}, // can't convert map to string
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, ok := GetFieldString(data, tt.field)
			if ok != tt.ok {
				t.Errorf("GetFieldString(%q) ok = %v, want %v", tt.field, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("GetFieldString(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestGetFieldFloat(t *testing.T) {
	data := map[string]interface{}{
		"float_val":  float64(42.5),
		"int_val":    float64(100),
		"string_num": "123.45",
		"string_bad": "not a number",
		"bool_val":   true,
	}

	tests := []struct {
		field string
		want  float64
		ok    bool
	}{
		{"float_val", 42.5, true},
		{"int_val", 100, true},
		{"string_num", 123.45, true},
		{"string_bad", 0, false},
		{"bool_val", 0, false},
		{"missing", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, ok := GetFieldFloat(data, tt.field)
			if ok != tt.ok {
				t.Errorf("GetFieldFloat(%q) ok = %v, want %v", tt.field, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("GetFieldFloat(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestJSONParser_RealWorldLogs(t *testing.T) {
	p := NewJSONParser()

	tests := []struct {
		name  string
		line  string
		check func(map[string]interface{}) bool
	}{
		{
			name: "traefik access log",
			line: `{"ClientAddr":"192.168.1.1:54321","ClientHost":"192.168.1.1","Duration":1234567,"OriginStatus":200,"RequestMethod":"GET","RequestPath":"/api/health","time":"2024-01-15T10:30:00Z"}`,
			check: func(data map[string]interface{}) bool {
				return data["ClientHost"] == "192.168.1.1" &&
					data["OriginStatus"] == float64(200) &&
					data["RequestMethod"] == "GET"
			},
		},
		{
			name: "application log",
			line: `{"timestamp":"2024-01-15T10:30:00Z","level":"info","event":"request_processed","user_id":"user_123","duration_ms":45}`,
			check: func(data map[string]interface{}) bool {
				return data["level"] == "info" &&
					data["event"] == "request_processed" &&
					data["user_id"] == "user_123"
			},
		},
		{
			name: "nested metrics",
			line: `{"timestamp":"2024-01-15T10:30:00Z","metrics":{"active_sessions":42,"memory_mb":512},"response":{"bytes":1024,"status":200}}`,
			check: func(data map[string]interface{}) bool {
				val, ok := GetFieldFloat(data, "metrics.active_sessions")
				if !ok || val != 42 {
					return false
				}
				val, ok = GetFieldFloat(data, "response.bytes")
				return ok && val == 1024
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.line)
			if result == nil {
				t.Fatal("Parse() = nil, want non-nil")
			}
			if !tt.check(result) {
				t.Errorf("Parse() check failed for %s", tt.name)
			}
		})
	}
}
