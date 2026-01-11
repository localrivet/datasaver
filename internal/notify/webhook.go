package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Notifier struct {
	webhookURL string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewNotifier(webhookURL string, logger *slog.Logger) *Notifier {
	if webhookURL == "" {
		return nil
	}

	return &Notifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

type WebhookPayload struct {
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	BackupID  string    `json:"backup_id,omitempty"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Details   Details   `json:"details,omitempty"`
}

type Details struct {
	Size     int64  `json:"size_bytes,omitempty"`
	Duration int64  `json:"duration_ms,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (n *Notifier) NotifySuccess(backupID string, size int64, duration time.Duration) {
	if n == nil {
		return
	}

	payload := WebhookPayload{
		Event:     "backup.completed",
		Timestamp: time.Now().UTC(),
		BackupID:  backupID,
		Status:    "success",
		Message:   fmt.Sprintf("Backup %s completed successfully", backupID),
		Details: Details{
			Size:     size,
			Duration: duration.Milliseconds(),
		},
	}

	n.send(payload)
}

func (n *Notifier) NotifyFailure(backupID string, err error) {
	if n == nil {
		return
	}

	payload := WebhookPayload{
		Event:     "backup.failed",
		Timestamp: time.Now().UTC(),
		BackupID:  backupID,
		Status:    "failure",
		Message:   fmt.Sprintf("Backup %s failed", backupID),
		Details: Details{
			Error: err.Error(),
		},
	}

	n.send(payload)
}

func (n *Notifier) NotifyAlert(message string) {
	if n == nil {
		return
	}

	payload := WebhookPayload{
		Event:     "backup.alert",
		Timestamp: time.Now().UTC(),
		Status:    "alert",
		Message:   message,
	}

	n.send(payload)
}

func (n *Notifier) send(payload WebhookPayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		n.logger.Error("failed to marshal webhook payload", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(data))
	if err != nil {
		n.logger.Error("failed to create webhook request", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "datasaver/1.0")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		n.logger.Error("failed to send webhook", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		n.logger.Warn("webhook returned error status", "status", resp.StatusCode)
	} else {
		n.logger.Debug("webhook sent successfully", "event", payload.Event)
	}
}
