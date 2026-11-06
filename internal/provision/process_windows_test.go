package provision

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWindowsCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	_, err := runCommand(ctx, "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 30"}, "", nil)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 6*time.Second {
		t.Fatalf("Cancellation failed: %v", err)
	}
}
