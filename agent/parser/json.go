// SPDX-License-Identifier: MIT

package parser

import (
	"encoding/json"
	"strconv"
	"strings"
)

// JSONParser parses JSON log lines.
type JSONParser struct{}

// NewJSONParser creates a new JSON parser.
func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

// Parse parses a JSON log line.
func (p *JSONParser) Parse(line string) map[string]interface{} {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil
	}
	return data
}

// GetField extracts a field from parsed data using dot notation.
// Supports nested fields like "metrics.active_sessions" or "response.bytes".
func GetField(data map[string]interface{}, field string) (interface{}, bool) {
	if data == nil {
		return nil, false
	}

	parts := strings.Split(field, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

// GetFieldString extracts a field as a string.
func GetFieldString(data map[string]interface{}, field string) (string, bool) {
	val, ok := GetField(data, field)
	if !ok {
		return "", false
	}

	switch v := val.(type) {
	case string:
		return v, true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case bool:
		return strconv.FormatBool(v), true
	default:
		return "", false
	}
}

// GetFieldFloat extracts a field as a float64.
func GetFieldFloat(data map[string]interface{}, field string) (float64, bool) {
	val, ok := GetField(data, field)
	if !ok {
		return 0, false
	}

	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
