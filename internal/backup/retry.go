package backup

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}

func WithRetry[T any](ctx context.Context, cfg RetryConfig, logger *slog.Logger, operation string, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T
	wait := cfg.InitialWait

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !isRetryable(err) {
			return zero, err
		}

		if attempt < cfg.MaxAttempts {
			logger.Warn("operation failed, retrying",
				"operation", operation,
				"attempt", attempt,
				"max_attempts", cfg.MaxAttempts,
				"error", err,
				"next_wait", wait,
			)

			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(wait):
			}

			wait = time.Duration(float64(wait) * cfg.Multiplier)
			if wait > cfg.MaxWait {
				wait = cfg.MaxWait
			}
		}
	}

	return zero, lastErr
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	errStr := err.Error()

	nonRetryableErrors := []string{
		"permission denied",
		"access denied",
		"authentication failed",
		"invalid password",
		"database does not exist",
		"role does not exist",
	}

	for _, s := range nonRetryableErrors {
		if contains(errStr, s) {
			return false
		}
	}

	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldSlice(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFoldSlice(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
