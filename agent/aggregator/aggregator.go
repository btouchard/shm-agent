// SPDX-License-Identifier: MIT

// Package aggregator provides metric aggregation with thread-safe operations.
package aggregator

import (
	"sync"
)

// MetricType represents the type of a metric.
type MetricType string

const (
	Counter MetricType = "counter"
	Gauge   MetricType = "gauge"
	Sum     MetricType = "sum"
	Set     MetricType = "set"
)

// MetricValue holds the current state of a metric.
type MetricValue struct {
	Type  MetricType
	Value float64            // Used for counter, gauge, sum
	Set   map[string]struct{} // Used for set (unique values)
}

// Aggregator manages metric aggregation.
type Aggregator struct {
	mu      sync.RWMutex
	metrics map[string]*MetricValue
}

// New creates a new Aggregator.
func New() *Aggregator {
	return &Aggregator{
		metrics: make(map[string]*MetricValue),
	}
}

// Register registers a metric with the given name and type.
// Must be called before using Inc, SetGauge, Add, or AddToSet.
func (a *Aggregator) Register(name string, metricType MetricType) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.metrics[name]; exists {
		return
	}

	mv := &MetricValue{Type: metricType}
	if metricType == Set {
		mv.Set = make(map[string]struct{})
	}
	a.metrics[name] = mv
}

// Inc increments a counter metric by 1.
func (a *Aggregator) Inc(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if m, ok := a.metrics[name]; ok && m.Type == Counter {
		m.Value++
	}
}

// SetGauge sets the value of a gauge metric.
func (a *Aggregator) SetGauge(name string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if m, ok := a.metrics[name]; ok && m.Type == Gauge {
		m.Value = value
	}
}

// Add adds a value to a sum metric.
func (a *Aggregator) Add(name string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if m, ok := a.metrics[name]; ok && m.Type == Sum {
		m.Value += value
	}
}

// AddToSet adds a value to a set metric.
func (a *Aggregator) AddToSet(name string, value string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if m, ok := a.metrics[name]; ok && m.Type == Set {
		m.Set[value] = struct{}{}
	}
}

// Snapshot returns the current metrics and resets counters, sums, and sets.
// Gauges are not reset.
func (a *Aggregator) Snapshot() map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make(map[string]interface{})

	for name, m := range a.metrics {
		switch m.Type {
		case Counter:
			result[name] = m.Value
			m.Value = 0 // Reset
		case Gauge:
			result[name] = m.Value
			// No reset for gauges
		case Sum:
			result[name] = m.Value
			m.Value = 0 // Reset
		case Set:
			result[name] = len(m.Set)
			m.Set = make(map[string]struct{}) // Reset
		}
	}

	return result
}

// Peek returns the current metrics without resetting.
func (a *Aggregator) Peek() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]interface{})

	for name, m := range a.metrics {
		switch m.Type {
		case Counter, Gauge, Sum:
			result[name] = m.Value
		case Set:
			result[name] = len(m.Set)
		}
	}

	return result
}

// Reset resets all metrics to their initial state.
func (a *Aggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, m := range a.metrics {
		m.Value = 0
		if m.Type == Set {
			m.Set = make(map[string]struct{})
		}
	}
}

// GetMetricType returns the type of a metric.
func (a *Aggregator) GetMetricType(name string) (MetricType, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if m, ok := a.metrics[name]; ok {
		return m.Type, true
	}
	return "", false
}
