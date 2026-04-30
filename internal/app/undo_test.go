package app_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
)

// captureToggler records each SetProxied invocation so the test can assert
// the undo path called the adapter with the snapshot's pre-toggle value.
type captureToggler struct {
	calls []captureCall
}

type captureCall struct {
	record  domain.Record
	proxied bool
}

func (c *captureToggler) SetProxied(_ context.Context, current domain.Record, proxied bool) error {
	c.calls = append(c.calls, captureCall{record: current, proxied: proxied})
	return nil
}

func TestUndoLast_NoAuditReturnsNothingToUndo(t *testing.T) {
	appCtx := &app.Context{
		Logger:  slog.Default(),
		Toggler: &captureToggler{},
	}
	_, err := app.UndoLast(context.Background(), appCtx, t.TempDir(), filepath.Join(t.TempDir(), "audit.jsonl"))
	if !errors.Is(err, app.ErrNothingToUndo) {
		t.Fatalf("err = %v, want ErrNothingToUndo", err)
	}
}

func TestUndoLast_RevertsLastAppliedEntry(t *testing.T) {
	stateDir := t.TempDir()
	snapDir := filepath.Join(stateDir, "snapshots")
	auditPath := filepath.Join(stateDir, "audit.jsonl")

	tog := &captureToggler{}
	appCtx := &app.Context{
		Logger:  slog.Default(),
		Toggler: tog,
	}

	// Seed a previous toggle: Proxied true -> false. ApplyToggle writes
	// the snapshot (of the BEFORE state) and an audit entry.
	target := domain.Record{
		ID:       "rec-abc",
		Type:     "A",
		Name:     "www",
		Content:  "1.2.3.4",
		ZoneID:   "zone-1",
		ZoneName: "example.com",
		Proxied:  true,
		TTL:      300,
	}
	if _, err := app.ApplyToggle(context.Background(), appCtx, target, false, snapDir, auditPath); err != nil {
		t.Fatalf("seed ApplyToggle: %v", err)
	}
	if len(tog.calls) != 1 {
		t.Fatalf("seed produced %d toggler calls, want 1", len(tog.calls))
	}
	if tog.calls[0].proxied != false {
		t.Fatalf("seed call proxied = %v, want false", tog.calls[0].proxied)
	}

	// Now undo. The toggler must be called AGAIN, this time with the
	// pre-toggle value (true). The "current" record passed to the toggler
	// MUST have proxied=AfterToggle (false) so the adapter sends the right
	// PUT body to Cloudflare.
	result, err := app.UndoLast(context.Background(), appCtx, snapDir, auditPath)
	if err != nil {
		t.Fatalf("UndoLast: %v", err)
	}

	if len(tog.calls) != 2 {
		t.Fatalf("after undo, total toggler calls = %d, want 2", len(tog.calls))
	}
	undoCall := tog.calls[1]
	if undoCall.proxied != true {
		t.Errorf("undo call proxied = %v, want true (revert to original)", undoCall.proxied)
	}
	if undoCall.record.Proxied != false {
		t.Errorf("undo call current.Proxied = %v, want false (post-toggle state)", undoCall.record.Proxied)
	}
	if undoCall.record.ID != "rec-abc" {
		t.Errorf("undo call record.ID = %q, want %q", undoCall.record.ID, "rec-abc")
	}
	if !result.Applied {
		t.Error("UndoResult.Applied = false, want true")
	}
	if result.SourceEntry.Name != "www" {
		t.Errorf("SourceEntry.Name = %q, want %q", result.SourceEntry.Name, "www")
	}
	if result.SnapshotPath == "" {
		t.Error("undo did not produce its own snapshot path")
	}
}

func TestUndoLast_AlsoUndoable(t *testing.T) {
	stateDir := t.TempDir()
	snapDir := filepath.Join(stateDir, "snapshots")
	auditPath := filepath.Join(stateDir, "audit.jsonl")

	tog := &captureToggler{}
	appCtx := &app.Context{
		Logger:  slog.Default(),
		Toggler: tog,
	}

	target := domain.Record{
		ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4",
		ZoneID: "z1", ZoneName: "example.com", Proxied: true, TTL: 300,
	}

	// Apply a toggle.
	if _, err := app.ApplyToggle(context.Background(), appCtx, target, false, snapDir, auditPath); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Undo it.
	if _, err := app.UndoLast(context.Background(), appCtx, snapDir, auditPath); err != nil {
		t.Fatalf("undo 1: %v", err)
	}
	// Undo the undo (should re-apply original).
	if _, err := app.UndoLast(context.Background(), appCtx, snapDir, auditPath); err != nil {
		t.Fatalf("undo 2: %v", err)
	}

	if len(tog.calls) != 3 {
		t.Fatalf("total toggler calls = %d, want 3", len(tog.calls))
	}
	if tog.calls[2].proxied != false {
		t.Errorf("third call (re-apply original) proxied = %v, want false", tog.calls[2].proxied)
	}
}

func TestUndoLast_SnapshotMissingErrors(t *testing.T) {
	stateDir := t.TempDir()
	snapDir := filepath.Join(stateDir, "snapshots")
	auditPath := filepath.Join(stateDir, "audit.jsonl")

	tog := &captureToggler{}
	appCtx := &app.Context{
		Logger:  slog.Default(),
		Toggler: tog,
	}

	target := domain.Record{
		ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4",
		ZoneID: "z1", ZoneName: "example.com", Proxied: true, TTL: 300,
	}
	if _, err := app.ApplyToggle(context.Background(), appCtx, target, false, snapDir, auditPath); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Sabotage: delete the snapshot file referenced by the latest audit entry.
	entries, _ := os.ReadDir(snapDir)
	if len(entries) == 0 {
		t.Fatal("no snapshot files produced by ApplyToggle")
	}
	if err := os.Remove(filepath.Join(snapDir, entries[0].Name())); err != nil {
		t.Fatalf("remove snapshot: %v", err)
	}

	_, err := app.UndoLast(context.Background(), appCtx, snapDir, auditPath)
	if err == nil {
		t.Fatal("UndoLast: expected error when snapshot missing, got nil")
	}
}
