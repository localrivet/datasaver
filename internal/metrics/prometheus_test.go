package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// resetRegistry clears the default prometheus registry for clean tests
func resetRegistry() {
	// Create a new registry for each test to avoid "duplicate metrics" errors
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
}

func TestNew(t *testing.T) {
	resetRegistry()

	m := New("test")
	if m == nil {
		t.Fatal("New() returned nil")
	}

	if m.backupDuration == nil {
		t.Error("backupDuration is nil")
	}
	if m.backupSize == nil {
		t.Error("backupSize is nil")
	}
	if m.backupTotal == nil {
		t.Error("backupTotal is nil")
	}
	if m.backupFailures == nil {
		t.Error("backupFailures is nil")
	}
	if m.lastBackupTime == nil {
		t.Error("lastBackupTime is nil")
	}
	if m.lastBackupSuccess == nil {
		t.Error("lastBackupSuccess is nil")
	}
	if m.storageUsed == nil {
		t.Error("storageUsed is nil")
	}
}

func TestNew_DefaultNamespace(t *testing.T) {
	resetRegistry()

	// When namespace is empty, should use "datasaver"
	m := New("")
	if m == nil {
		t.Fatal("New() returned nil")
	}
}

func TestMetrics_RecordBackupSuccess(t *testing.T) {
	resetRegistry()

	m := New("test_success")

	// Record a successful backup
	m.RecordBackupSuccess(5*time.Second, 1024*1024)

	// We can't easily verify the values without exposing the metrics
	// but at least we verify no panic occurs
}

func TestMetrics_RecordBackupFailure(t *testing.T) {
	resetRegistry()

	m := New("test_failure")

	// Record a failed backup
	m.RecordBackupFailure()

	// Verify no panic
}

func TestMetrics_SetStorageUsed(t *testing.T) {
	resetRegistry()

	m := New("test_storage")

	m.SetStorageUsed(1024 * 1024 * 100) // 100MB

	// Verify no panic
}

func TestMetrics_MultipleOperations(t *testing.T) {
	resetRegistry()

	m := New("test_multi")

	// Simulate multiple backup cycles
	for i := 0; i < 5; i++ {
		m.RecordBackupSuccess(time.Duration(i)*time.Second, int64(i)*1024)
	}

	m.RecordBackupFailure()
	m.RecordBackupFailure()

	m.SetStorageUsed(5 * 1024)

	// All operations should complete without panic
}

func TestHandler(t *testing.T) {
	h := Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}

	// Verify it's a valid HTTP handler
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Response should contain prometheus metrics format
	body := w.Body.String()
	if !strings.Contains(body, "go_") {
		t.Error("Expected prometheus metrics in response")
	}
}

func TestMetrics_HistogramBuckets(t *testing.T) {
	resetRegistry()

	m := New("test_buckets")

	// Record various durations to hit different buckets
	durations := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
		60 * time.Second,
	}

	for _, d := range durations {
		m.RecordBackupSuccess(d, 1024)
	}

	// Verify no panic with various bucket values
}

func TestMetrics_LargeValues(t *testing.T) {
	resetRegistry()

	m := New("test_large")

	// Test with large values (10TB backup, 1 hour duration)
	m.RecordBackupSuccess(time.Hour, 10*1024*1024*1024*1024)
	m.SetStorageUsed(100 * 1024 * 1024 * 1024 * 1024) // 100TB

	// Should handle large values without overflow
}

func TestMetrics_ZeroValues(t *testing.T) {
	resetRegistry()

	m := New("test_zero")

	// Test with zero values
	m.RecordBackupSuccess(0, 0)
	m.SetStorageUsed(0)

	// Should handle zero values
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	resetRegistry()

	m := New("test_concurrent")

	done := make(chan bool, 10)

	// Simulate concurrent metric updates
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				if j%2 == 0 {
					m.RecordBackupSuccess(time.Second, 1024)
				} else {
					m.RecordBackupFailure()
				}
				m.SetStorageUsed(int64(j * 1024))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic, concurrent access is safe
}
