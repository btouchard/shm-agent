// SPDX-License-Identifier: MIT

// Package parser provides log line parsing functionality.
package parser

// Parser is the interface for log line parsers.
type Parser interface {
	// Parse parses a log line and returns extracted fields.
	// Returns nil if the line cannot be parsed.
	Parse(line string) map[string]interface{}
}

// New creates a parser based on the format.
func New(format string, pattern string) (Parser, error) {
	switch format {
	case "json":
		return NewJSONParser(), nil
	case "regex":
		return NewRegexParser(pattern)
	default:
		return nil, &UnsupportedFormatError{Format: format}
	}
}

// UnsupportedFormatError is returned when an unsupported format is requested.
type UnsupportedFormatError struct {
	Format string
}

func (e *UnsupportedFormatError) Error() string {
	return "unsupported format: " + e.Format
}
