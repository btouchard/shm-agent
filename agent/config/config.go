// SPDX-License-Identifier: MIT

// Package config provides configuration parsing and validation for shm-agent.
package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main agent configuration.
type Config struct {
	ServerURL    string        `yaml:"server_url"`
	IdentityFile string        `yaml:"identity_file"`
	AppName      string        `yaml:"app_name"`
	AppVersion   string        `yaml:"app_version"`
	Environment  string        `yaml:"environment"`
	Interval     time.Duration `yaml:"interval"`
	Sources      []Source      `yaml:"sources"`
}

// Source represents a log source configuration.
type Source struct {
	Path    string   `yaml:"path"`
	Format  string   `yaml:"format"` // "json" or "regex"
	Pattern string   `yaml:"pattern"` // regex pattern (only for format: regex)
	Metrics []Metric `yaml:"metrics"`
}

// Metric represents a metric extraction configuration.
type Metric struct {
	Name    string  `yaml:"name"`
	Type    string  `yaml:"type"` // "counter", "gauge", "sum", "set"
	Match   *Match  `yaml:"match,omitempty"`
	Extract *Extract `yaml:"extract,omitempty"`
}

// Match represents a matching condition.
type Match struct {
	Field    string   `yaml:"field"`
	Equals   string   `yaml:"equals,omitempty"`
	In       []string `yaml:"in,omitempty"`
	Regex    string   `yaml:"regex,omitempty"`
	Contains string   `yaml:"contains,omitempty"`
}

// Extract represents a field extraction configuration.
type Extract struct {
	Field string `yaml:"field"`
}

// Load reads and parses a configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	return Parse(data)
}

// Parse parses configuration from YAML data.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if err := cfg.setDefaults(); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults sets default values for configuration fields.
func (c *Config) setDefaults() error {
	if c.IdentityFile == "" {
		c.IdentityFile = "./shm_identity.json"
	}

	if c.Interval == 0 {
		c.Interval = 60 * time.Second
	}

	if c.Environment == "" {
		c.Environment = "production"
	}

	return nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.ServerURL == "" {
		return fmt.Errorf("server_url is required")
	}

	if c.AppName == "" {
		return fmt.Errorf("app_name is required")
	}

	if c.AppVersion == "" {
		return fmt.Errorf("app_version is required")
	}

	if c.Interval < time.Second {
		return fmt.Errorf("interval must be at least 1 second")
	}

	if len(c.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}

	for i, src := range c.Sources {
		if err := src.Validate(); err != nil {
			return fmt.Errorf("source[%d]: %w", i, err)
		}
	}

	return nil
}

// Validate validates a source configuration.
func (s *Source) Validate() error {
	if s.Path == "" {
		return fmt.Errorf("path is required")
	}

	if s.Format == "" {
		return fmt.Errorf("format is required")
	}

	if s.Format != "json" && s.Format != "regex" {
		return fmt.Errorf("format must be 'json' or 'regex', got '%s'", s.Format)
	}

	if s.Format == "regex" && s.Pattern == "" {
		return fmt.Errorf("pattern is required for regex format")
	}

	if s.Format == "regex" {
		if _, err := regexp.Compile(s.Pattern); err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	if len(s.Metrics) == 0 {
		return fmt.Errorf("at least one metric is required")
	}

	for i, m := range s.Metrics {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("metric[%d]: %w", i, err)
		}
	}

	return nil
}

// Validate validates a metric configuration.
func (m *Metric) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}

	validTypes := map[string]bool{
		"counter": true,
		"gauge":   true,
		"sum":     true,
		"set":     true,
	}

	if !validTypes[m.Type] {
		return fmt.Errorf("type must be one of: counter, gauge, sum, set; got '%s'", m.Type)
	}

	// sum, gauge, and set require extract (unless counter with no value extraction)
	if (m.Type == "sum" || m.Type == "gauge" || m.Type == "set") && m.Extract == nil {
		return fmt.Errorf("extract is required for type '%s'", m.Type)
	}

	if m.Match != nil {
		if err := m.Match.Validate(); err != nil {
			return fmt.Errorf("match: %w", err)
		}
	}

	return nil
}

// Validate validates a match configuration.
func (m *Match) Validate() error {
	if m.Field == "" {
		return fmt.Errorf("field is required")
	}

	conditions := 0
	if m.Equals != "" {
		conditions++
	}
	if len(m.In) > 0 {
		conditions++
	}
	if m.Regex != "" {
		conditions++
	}
	if m.Contains != "" {
		conditions++
	}

	if conditions == 0 {
		return fmt.Errorf("at least one condition (equals, in, regex, contains) is required")
	}

	if conditions > 1 {
		return fmt.Errorf("only one condition (equals, in, regex, contains) is allowed")
	}

	if m.Regex != "" {
		if _, err := regexp.Compile(m.Regex); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}

	return nil
}
