// SPDX-License-Identifier: MIT

package parser

import (
	"fmt"
	"regexp"
)

// RegexParser parses log lines using a regular expression with named groups.
type RegexParser struct {
	re         *regexp.Regexp
	groupNames []string
}

// NewRegexParser creates a new regex parser.
func NewRegexParser(pattern string) (*RegexParser, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return &RegexParser{
		re:         re,
		groupNames: re.SubexpNames(),
	}, nil
}

// Parse parses a log line using the regex pattern.
// Returns a map of named group names to their matched values.
func (p *RegexParser) Parse(line string) map[string]interface{} {
	matches := p.re.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	result := make(map[string]interface{})
	for i, name := range p.groupNames {
		if name != "" && i < len(matches) {
			result[name] = matches[i]
		}
	}

	// Return nil if no named groups matched
	if len(result) == 0 {
		return nil
	}

	return result
}

// Pattern returns the regex pattern string.
func (p *RegexParser) Pattern() string {
	return p.re.String()
}

// GroupNames returns the list of named capture groups.
func (p *RegexParser) GroupNames() []string {
	names := make([]string, 0, len(p.groupNames))
	for _, name := range p.groupNames {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}
