package app

import (
	"context"
	"testing"
	"time"

	"github.com/keyxmakerx/chronicle/internal/config"
)

// TestNew_ShutdownContextCancelsOnShutdown pins the contract that App
// exposes a shutdown context background work can select on, and that
// ShutdownCancel actually cancels it. Before this existed, startup
// background jobs (e.g. the media content-hash backfill) were wired to
// context.Background() and had no way to stop when the server shut down,
// keeping them working against a closing database (#711).
func TestNew_ShutdownContextCancelsOnShutdown(t *testing.T) {
	a, err := New(&config.Config{}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if a.ShutdownCtx == nil {
		t.Fatal("App.ShutdownCtx is nil; background jobs have no shutdown signal")
	}
	if a.ShutdownCancel == nil {
		t.Fatal("App.ShutdownCancel is nil; nothing can cancel ShutdownCtx")
	}

	select {
	case <-a.ShutdownCtx.Done():
		t.Fatal("ShutdownCtx already canceled before ShutdownCancel was called")
	default:
	}

	a.ShutdownCancel()

	select {
	case <-a.ShutdownCtx.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("ShutdownCtx not canceled after ShutdownCancel()")
	}
	if !errorsIsCanceled(a.ShutdownCtx.Err()) {
		t.Errorf("ShutdownCtx.Err() = %v, want context.Canceled", a.ShutdownCtx.Err())
	}
}

func errorsIsCanceled(err error) bool {
	return err == context.Canceled
}
