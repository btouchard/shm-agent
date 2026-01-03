// SPDX-License-Identifier: MIT

package matcher

import (
	"testing"

	"github.com/kolapsis/shm-agent/agent/config"
)

func TestMatcher_Equals(t *testing.T) {
	m, err := New(&config.Match{
		Field:  "level",
		Equals: "error",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		data map[string]interface{}
		want bool
	}{
		{map[string]interface{}{"level": "error"}, true},
		{map[string]interface{}{"level": "info"}, false},
		{map[string]interface{}{"level": "ERROR"}, false}, // Case sensitive
		{map[string]interface{}{"other": "error"}, false},
		{map[string]interface{}{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := m.Match(tt.data); got != tt.want {
			t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatcher_In(t *testing.T) {
	m, err := New(&config.Match{
		Field: "level",
		In:    []string{"error", "fatal", "critical"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		data map[string]interface{}
		want bool
	}{
		{map[string]interface{}{"level": "error"}, true},
		{map[string]interface{}{"level": "fatal"}, true},
		{map[string]interface{}{"level": "critical"}, true},
		{map[string]interface{}{"level": "info"}, false},
		{map[string]interface{}{"level": "warn"}, false},
		{map[string]interface{}{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := m.Match(tt.data); got != tt.want {
			t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatcher_Regex(t *testing.T) {
	m, err := New(&config.Match{
		Field: "status",
		Regex: `^5\d{2}$`,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		data map[string]interface{}
		want bool
	}{
		{map[string]interface{}{"status": "500"}, true},
		{map[string]interface{}{"status": "502"}, true},
		{map[string]interface{}{"status": "599"}, true},
		{map[string]interface{}{"status": "200"}, false},
		{map[string]interface{}{"status": "404"}, false},
		{map[string]interface{}{"status": "5000"}, false}, // Too many digits
		{map[string]interface{}{"status": "50"}, false},   // Too few digits
		{map[string]interface{}{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := m.Match(tt.data); got != tt.want {
			t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatcher_Contains(t *testing.T) {
	m, err := New(&config.Match{
		Field:    "message",
		Contains: "error",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		data map[string]interface{}
		want bool
	}{
		{map[string]interface{}{"message": "An error occurred"}, true},
		{map[string]interface{}{"message": "error at line 5"}, true},
		{map[string]interface{}{"message": "Something went wrong with error handling"}, true},
		{map[string]interface{}{"message": "All good"}, false},
		{map[string]interface{}{"message": "ERROR"}, false}, // Case sensitive
		{map[string]interface{}{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := m.Match(tt.data); got != tt.want {
			t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatcher_AlwaysMatches(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) error = %v", err)
	}

	if !m.AlwaysMatches() {
		t.Error("AlwaysMatches() = false, want true")
	}

	tests := []struct {
		data map[string]interface{}
		want bool
	}{
		{map[string]interface{}{"any": "value"}, true},
		{map[string]interface{}{}, true},
		{nil, true},
	}

	for _, tt := range tests {
		if got := m.Match(tt.data); got != tt.want {
			t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatcher_NestedField(t *testing.T) {
	m, err := New(&config.Match{
		Field:  "response.status",
		Equals: "200",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		data map[string]interface{}
		want bool
	}{
		{map[string]interface{}{
			"response": map[string]interface{}{
				"status": "200",
			},
		}, true},
		{map[string]interface{}{
			"response": map[string]interface{}{
				"status": "404",
			},
		}, false},
		{map[string]interface{}{
			"response": map[string]interface{}{},
		}, false},
		{map[string]interface{}{}, false},
	}

	for _, tt := range tests {
		if got := m.Match(tt.data); got != tt.want {
			t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatcher_NumericFieldAsString(t *testing.T) {
	// When JSON is parsed, numbers are float64
	// The matcher should convert them to strings for comparison
	m, err := New(&config.Match{
		Field:  "status",
		Equals: "200",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		data map[string]interface{}
		want bool
	}{
		{map[string]interface{}{"status": float64(200)}, true},
		{map[string]interface{}{"status": float64(404)}, false},
		{map[string]interface{}{"status": "200"}, true},
	}

	for _, tt := range tests {
		if got := m.Match(tt.data); got != tt.want {
			t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatcher_InvalidRegex(t *testing.T) {
	_, err := New(&config.Match{
		Field: "status",
		Regex: "[invalid",
	})
	if err == nil {
		t.Error("New() should return error for invalid regex")
	}
}

func TestMatcher_Field(t *testing.T) {
	m, err := New(&config.Match{
		Field:  "level",
		Equals: "error",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if m.Field() != "level" {
		t.Errorf("Field() = %q, want %q", m.Field(), "level")
	}
}

func TestMatcher_RegexWithNumbers(t *testing.T) {
	m, err := New(&config.Match{
		Field: "status",
		Regex: `^[45]\d{2}$`,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name string
		data map[string]interface{}
		want bool
	}{
		{"400", map[string]interface{}{"status": float64(400)}, true},
		{"404", map[string]interface{}{"status": float64(404)}, true},
		{"500", map[string]interface{}{"status": float64(500)}, true},
		{"503", map[string]interface{}{"status": float64(503)}, true},
		{"200", map[string]interface{}{"status": float64(200)}, false},
		{"301", map[string]interface{}{"status": float64(301)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.Match(tt.data); got != tt.want {
				t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestMatcher_InWithNumbers(t *testing.T) {
	m, err := New(&config.Match{
		Field: "status",
		In:    []string{"500", "502", "503"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name string
		data map[string]interface{}
		want bool
	}{
		{"500 float", map[string]interface{}{"status": float64(500)}, true},
		{"502 float", map[string]interface{}{"status": float64(502)}, true},
		{"503 float", map[string]interface{}{"status": float64(503)}, true},
		{"500 string", map[string]interface{}{"status": "500"}, true},
		{"200 float", map[string]interface{}{"status": float64(200)}, false},
		{"501 float", map[string]interface{}{"status": float64(501)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.Match(tt.data); got != tt.want {
				t.Errorf("Match(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}
