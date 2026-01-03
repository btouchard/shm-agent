// SPDX-License-Identifier: MIT

// Package matcher provides field matching logic for log lines.
package matcher

import (
	"regexp"
	"strings"

	"github.com/kolapsis/shm-agent/agent/config"
	"github.com/kolapsis/shm-agent/agent/parser"
)

// Matcher checks if parsed data matches configured conditions.
type Matcher struct {
	field    string
	equals   string
	in       map[string]struct{}
	regex    *regexp.Regexp
	contains string
	always   bool // true if no conditions (always matches)
}

// New creates a new Matcher from a config.Match.
// If match is nil, creates a matcher that always matches.
func New(match *config.Match) (*Matcher, error) {
	if match == nil {
		return &Matcher{always: true}, nil
	}

	m := &Matcher{
		field:    match.Field,
		equals:   match.Equals,
		contains: match.Contains,
	}

	if len(match.In) > 0 {
		m.in = make(map[string]struct{}, len(match.In))
		for _, v := range match.In {
			m.in[v] = struct{}{}
		}
	}

	if match.Regex != "" {
		re, err := regexp.Compile(match.Regex)
		if err != nil {
			return nil, err
		}
		m.regex = re
	}

	return m, nil
}

// Match checks if the parsed data matches the conditions.
func (m *Matcher) Match(data map[string]interface{}) bool {
	if m.always {
		return true
	}

	if data == nil {
		return false
	}

	// Get field value as string
	val, ok := parser.GetFieldString(data, m.field)
	if !ok {
		return false
	}

	// Check conditions
	if m.equals != "" {
		return val == m.equals
	}

	if m.in != nil {
		_, exists := m.in[val]
		return exists
	}

	if m.regex != nil {
		return m.regex.MatchString(val)
	}

	if m.contains != "" {
		return strings.Contains(val, m.contains)
	}

	return false
}

// Field returns the field name this matcher checks.
func (m *Matcher) Field() string {
	return m.field
}

// AlwaysMatches returns true if this matcher has no conditions.
func (m *Matcher) AlwaysMatches() bool {
	return m.always
}
