package notify

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewNotifier_EmptyURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier("", logger)
	if n != nil {
		t.Error("NewNotifier with empty URL should return nil")
	}
}

func TestNewNotifier_ValidURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier("https://example.com/webhook", logger)
	if n == nil {
		t.Error("NewNotifier with valid URL should not return nil")
	}
}

func TestNotifier_NotifySuccess(t *testing.T) {
	var receivedPayload WebhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("User-Agent") != "datasaver/1.0" {
			t.Errorf("Expected User-Agent datasaver/1.0, got %s", r.Header.Get("User-Agent"))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Fatalf("Failed to unmarshal payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier(server.URL, logger)

	n.NotifySuccess("backup_123", 1024*1024, 5*time.Second)

	// Give time for async send
	time.Sleep(100 * time.Millisecond)

	if receivedPayload.Event != "backup.completed" {
		t.Errorf("Expected event backup.completed, got %s", receivedPayload.Event)
	}

	if receivedPayload.Status != "success" {
		t.Errorf("Expected status success, got %s", receivedPayload.Status)
	}

	if receivedPayload.BackupID != "backup_123" {
		t.Errorf("Expected backup_id backup_123, got %s", receivedPayload.BackupID)
	}

	if receivedPayload.Details.Size != 1024*1024 {
		t.Errorf("Expected size 1048576, got %d", receivedPayload.Details.Size)
	}

	if receivedPayload.Details.Duration != 5000 {
		t.Errorf("Expected duration 5000ms, got %d", receivedPayload.Details.Duration)
	}
}

func TestNotifier_NotifyFailure(t *testing.T) {
	var receivedPayload WebhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier(server.URL, logger)

	testErr := &testError{msg: "database connection failed"}
	n.NotifyFailure("backup_456", testErr)

	time.Sleep(100 * time.Millisecond)

	if receivedPayload.Event != "backup.failed" {
		t.Errorf("Expected event backup.failed, got %s", receivedPayload.Event)
	}

	if receivedPayload.Status != "failure" {
		t.Errorf("Expected status failure, got %s", receivedPayload.Status)
	}

	if receivedPayload.Details.Error != "database connection failed" {
		t.Errorf("Expected error message, got %s", receivedPayload.Details.Error)
	}
}

func TestNotifier_NotifyAlert(t *testing.T) {
	var receivedPayload WebhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier(server.URL, logger)

	n.NotifyAlert("No backup in last 26 hours!")

	time.Sleep(100 * time.Millisecond)

	if receivedPayload.Event != "backup.alert" {
		t.Errorf("Expected event backup.alert, got %s", receivedPayload.Event)
	}

	if receivedPayload.Status != "alert" {
		t.Errorf("Expected status alert, got %s", receivedPayload.Status)
	}

	if receivedPayload.Message != "No backup in last 26 hours!" {
		t.Errorf("Expected alert message, got %s", receivedPayload.Message)
	}
}

func TestNotifier_NilSafe(t *testing.T) {
	var n *Notifier = nil

	// These should not panic
	n.NotifySuccess("test", 0, 0)
	n.NotifyFailure("test", &testError{msg: "test"})
	n.NotifyAlert("test")
}

func TestNotifier_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier(server.URL, logger)

	// Should not panic on server error
	n.NotifySuccess("test", 100, time.Second)

	time.Sleep(100 * time.Millisecond)
}

func TestNotifier_InvalidURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	n := NewNotifier("http://invalid-host-that-does-not-exist.local:9999", logger)

	// Should not panic on connection error
	n.NotifySuccess("test", 100, time.Second)
}

func TestWebhookPayload_JSON(t *testing.T) {
	payload := WebhookPayload{
		Event:     "backup.completed",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		BackupID:  "backup_123",
		Status:    "success",
		Message:   "Backup completed",
		Details: Details{
			Size:     1024,
			Duration: 5000,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded WebhookPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Event != payload.Event {
		t.Errorf("Event mismatch")
	}

	if decoded.BackupID != payload.BackupID {
		t.Errorf("BackupID mismatch")
	}
}

// Helper error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
