// SPDX-License-Identifier: MIT

package aggregator

import (
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	a := New()
	a.Register("requests", Counter)

	a.Inc("requests")
	a.Inc("requests")
	a.Inc("requests")

	metrics := a.Peek()
	if v := metrics["requests"].(float64); v != 3 {
		t.Errorf("requests = %v, want 3", v)
	}
}

func TestCounterReset(t *testing.T) {
	a := New()
	a.Register("requests", Counter)

	a.Inc("requests")
	a.Inc("requests")

	// Snapshot should reset
	metrics := a.Snapshot()
	if v := metrics["requests"].(float64); v != 2 {
		t.Errorf("snapshot requests = %v, want 2", v)
	}

	// After snapshot, should be 0
	metrics = a.Peek()
	if v := metrics["requests"].(float64); v != 0 {
		t.Errorf("after snapshot requests = %v, want 0", v)
	}
}

func TestGauge(t *testing.T) {
	a := New()
	a.Register("active_sessions", Gauge)

	a.SetGauge("active_sessions", 10)
	a.SetGauge("active_sessions", 15)
	a.SetGauge("active_sessions", 12)

	metrics := a.Peek()
	if v := metrics["active_sessions"].(float64); v != 12 {
		t.Errorf("active_sessions = %v, want 12", v)
	}
}

func TestGaugeNoReset(t *testing.T) {
	a := New()
	a.Register("active_sessions", Gauge)

	a.SetGauge("active_sessions", 10)

	// Snapshot should NOT reset gauge
	a.Snapshot()

	metrics := a.Peek()
	if v := metrics["active_sessions"].(float64); v != 10 {
		t.Errorf("after snapshot active_sessions = %v, want 10", v)
	}
}

func TestSum(t *testing.T) {
	a := New()
	a.Register("total_bytes", Sum)

	a.Add("total_bytes", 100)
	a.Add("total_bytes", 250.5)
	a.Add("total_bytes", 49.5)

	metrics := a.Peek()
	if v := metrics["total_bytes"].(float64); v != 400 {
		t.Errorf("total_bytes = %v, want 400", v)
	}
}

func TestSumReset(t *testing.T) {
	a := New()
	a.Register("total_bytes", Sum)

	a.Add("total_bytes", 100)
	a.Add("total_bytes", 200)

	metrics := a.Snapshot()
	if v := metrics["total_bytes"].(float64); v != 300 {
		t.Errorf("snapshot total_bytes = %v, want 300", v)
	}

	metrics = a.Peek()
	if v := metrics["total_bytes"].(float64); v != 0 {
		t.Errorf("after snapshot total_bytes = %v, want 0", v)
	}
}

func TestSet(t *testing.T) {
	a := New()
	a.Register("unique_users", Set)

	a.AddToSet("unique_users", "user1")
	a.AddToSet("unique_users", "user2")
	a.AddToSet("unique_users", "user1") // Duplicate
	a.AddToSet("unique_users", "user3")

	metrics := a.Peek()
	if v := metrics["unique_users"].(int); v != 3 {
		t.Errorf("unique_users = %v, want 3", v)
	}
}

func TestSetReset(t *testing.T) {
	a := New()
	a.Register("unique_ips", Set)

	a.AddToSet("unique_ips", "192.168.1.1")
	a.AddToSet("unique_ips", "192.168.1.2")

	metrics := a.Snapshot()
	if v := metrics["unique_ips"].(int); v != 2 {
		t.Errorf("snapshot unique_ips = %v, want 2", v)
	}

	metrics = a.Peek()
	if v := metrics["unique_ips"].(int); v != 0 {
		t.Errorf("after snapshot unique_ips = %v, want 0", v)
	}
}

func TestConcurrentAccess(t *testing.T) {
	a := New()
	a.Register("requests", Counter)
	a.Register("bytes", Sum)
	a.Register("users", Set)

	var wg sync.WaitGroup
	numGoroutines := 100
	opsPerGoroutine := 1000

	// Counters
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				a.Inc("requests")
			}
		}()
	}

	// Sums
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				a.Add("bytes", 1)
			}
		}()
	}

	// Sets (fewer to avoid memory issues)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				a.AddToSet("users", string(rune('A'+id)))
			}
		}(i)
	}

	wg.Wait()

	metrics := a.Peek()

	expectedRequests := float64(numGoroutines * opsPerGoroutine)
	if v := metrics["requests"].(float64); v != expectedRequests {
		t.Errorf("requests = %v, want %v", v, expectedRequests)
	}

	expectedBytes := float64(numGoroutines * opsPerGoroutine)
	if v := metrics["bytes"].(float64); v != expectedBytes {
		t.Errorf("bytes = %v, want %v", v, expectedBytes)
	}

	if v := metrics["users"].(int); v != 10 {
		t.Errorf("users = %v, want 10", v)
	}
}

func TestUnregisteredMetric(t *testing.T) {
	a := New()

	// These should not panic
	a.Inc("unknown")
	a.SetGauge("unknown", 10)
	a.Add("unknown", 100)
	a.AddToSet("unknown", "value")

	metrics := a.Peek()
	if len(metrics) != 0 {
		t.Errorf("expected empty metrics, got %d entries", len(metrics))
	}
}

func TestWrongMetricType(t *testing.T) {
	a := New()
	a.Register("counter", Counter)
	a.Register("gauge", Gauge)

	// Try to use counter as gauge - should be ignored
	a.SetGauge("counter", 10)
	a.Inc("counter")

	metrics := a.Peek()
	if v := metrics["counter"].(float64); v != 1 {
		t.Errorf("counter = %v, want 1", v)
	}

	// Try to use gauge as counter - should be ignored
	a.Inc("gauge")
	a.SetGauge("gauge", 10)

	// Need to call Peek() again to get updated values
	metrics = a.Peek()
	if v := metrics["gauge"].(float64); v != 10 {
		t.Errorf("gauge = %v, want 10", v)
	}
}

func TestReset(t *testing.T) {
	a := New()
	a.Register("counter", Counter)
	a.Register("gauge", Gauge)
	a.Register("sum", Sum)
	a.Register("set", Set)

	a.Inc("counter")
	a.SetGauge("gauge", 10)
	a.Add("sum", 100)
	a.AddToSet("set", "value")

	a.Reset()

	metrics := a.Peek()
	if v := metrics["counter"].(float64); v != 0 {
		t.Errorf("counter = %v, want 0", v)
	}
	if v := metrics["gauge"].(float64); v != 0 {
		t.Errorf("gauge = %v, want 0", v)
	}
	if v := metrics["sum"].(float64); v != 0 {
		t.Errorf("sum = %v, want 0", v)
	}
	if v := metrics["set"].(int); v != 0 {
		t.Errorf("set = %v, want 0", v)
	}
}

func TestGetMetricType(t *testing.T) {
	a := New()
	a.Register("counter", Counter)
	a.Register("gauge", Gauge)

	typ, ok := a.GetMetricType("counter")
	if !ok || typ != Counter {
		t.Errorf("GetMetricType(counter) = %v, %v; want Counter, true", typ, ok)
	}

	typ, ok = a.GetMetricType("unknown")
	if ok {
		t.Errorf("GetMetricType(unknown) = %v, %v; want '', false", typ, ok)
	}
}

func TestLargeNumbers(t *testing.T) {
	a := New()
	a.Register("counter", Counter)
	a.Register("sum", Sum)

	// Test large increments
	for i := 0; i < 1000000; i++ {
		a.Inc("counter")
	}

	metrics := a.Peek()
	if v := metrics["counter"].(float64); v != 1000000 {
		t.Errorf("counter = %v, want 1000000", v)
	}

	// Test large sums
	a.Add("sum", 1e15)
	a.Add("sum", 1e15)

	metrics = a.Peek()
	if v := metrics["sum"].(float64); v != 2e15 {
		t.Errorf("sum = %v, want 2e15", v)
	}
}

func TestRegisterTwice(t *testing.T) {
	a := New()
	a.Register("metric", Counter)
	a.Inc("metric")

	// Registering again should not reset
	a.Register("metric", Counter)

	metrics := a.Peek()
	if v := metrics["metric"].(float64); v != 1 {
		t.Errorf("metric = %v, want 1", v)
	}
}
