package app_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/infra/audit"
)

// fakeListerForToggle stubs the listing port. ListZones returns a fixed set
// of zones; ListRecords returns recordsByZone keyed on zone ID.
type fakeListerForToggle struct {
	zones         []domain.Zone
	recordsByZone map[string][]domain.Record
}

func (f *fakeListerForToggle) ListZones(_ context.Context) ([]domain.Zone, error) {
	return f.zones, nil
}

func (f *fakeListerForToggle) ListRecords(_ context.Context, zoneID string) ([]domain.Record, error) {
	return f.recordsByZone[zoneID], nil
}

// fakeToggler captures the call args; returns the configured error.
type fakeToggler struct {
	called    bool
	gotRecord domain.Record
	gotProxy  bool
	err       error
}

func (f *fakeToggler) SetProxied(_ context.Context, current domain.Record, proxied bool) error {
	f.called = true
	f.gotRecord = current
	f.gotProxy = proxied
	return f.err
}

func newAppCtx(lister domain.RecordLister, toggler domain.RecordToggler) *app.Context {
	return &app.Context{
		Logger:  slog.Default(),
		Lister:  lister,
		Toggler: toggler,
	}
}

func sampleZoneAndRecords() ([]domain.Zone, map[string][]domain.Record) {
	zones := []domain.Zone{{ID: "z1", Name: "example.com", Status: "active"}}
	records := map[string][]domain.Record{
		"z1": {
			{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", Proxied: true, TTL: 300},
			{ID: "r2", Type: "AAAA", Name: "www", Content: "::1", Proxied: false, TTL: 300},
		},
	}
	return zones, records
}

func TestToggleProxy_DryRunNeverCallsToggler(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	tog := &fakeToggler{}
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, tog)

	res, err := app.ToggleProxy(context.Background(), ctx, "example.com", "www", "A", false, false /* apply */, t.TempDir(), filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatalf("ToggleProxy: %v", err)
	}

	if tog.called {
		t.Error("Toggler.SetProxied called on dry-run path; must not")
	}
	if res.Applied {
		t.Error("ToggleResult.Applied = true on dry-run; must be false")
	}
	if res.SnapshotPath != "" {
		t.Errorf("SnapshotPath set on dry-run: %q", res.SnapshotPath)
	}
	if res.Diff.FromProxied != true || res.Diff.ToProxied != false {
		t.Errorf("Diff = %+v, want FromProxied=true ToProxied=false", res.Diff)
	}
}

func TestToggleProxy_DryRunNoOpReturnsImmediately(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	tog := &fakeToggler{}
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, tog)

	// Request proxied=true while record is already proxied=true → no-op.
	res, err := app.ToggleProxy(context.Background(), ctx, "example.com", "www", "A", true, true /* apply */, t.TempDir(), filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatalf("ToggleProxy: %v", err)
	}
	if !res.Diff.IsNoOp() {
		t.Errorf("Diff should be no-op; got %+v", res.Diff)
	}
	if tog.called {
		t.Error("Toggler.SetProxied called on no-op apply path; must not")
	}
	if res.Applied {
		t.Error("Applied = true on no-op; must be false")
	}
}

func TestToggleProxy_ApplyWritesSnapshotBeforeAPICall(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	tog := &fakeToggler{err: errors.New("boom")}
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, tog)

	snapDir := t.TempDir()
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")

	_, err := app.ToggleProxy(context.Background(), ctx, "example.com", "www", "A", false, true, snapDir, auditPath)
	if err == nil {
		t.Fatal("expected error from failing toggler, got nil")
	}

	// Snapshot dir MUST contain a file even though the API call failed.
	entries, _ := os.ReadDir(snapDir)
	if len(entries) != 1 {
		t.Fatalf("snapshot dir has %d entries, want 1 (snapshot must be written before API call)", len(entries))
	}
}

func TestToggleProxy_ApplyAppendsAuditOnSuccess(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	tog := &fakeToggler{}
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, tog)

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	res, err := app.ToggleProxy(context.Background(), ctx, "example.com", "www", "A", false, true, t.TempDir(), auditPath)
	if err != nil {
		t.Fatalf("ToggleProxy: %v", err)
	}
	if !res.Applied {
		t.Error("Applied = false on success; must be true")
	}
	if !tog.called {
		t.Error("Toggler.SetProxied not called on apply path")
	}
	if tog.gotProxy != false {
		t.Errorf("toggler received proxied=%v, want false", tog.gotProxy)
	}

	// Audit file must have exactly one entry, applied=true, no error.
	f, _ := os.Open(auditPath)
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines int
	var lastEntry audit.Entry
	for sc.Scan() {
		lines++
		_ = json.Unmarshal(sc.Bytes(), &lastEntry)
	}
	if lines != 1 {
		t.Fatalf("audit lines = %d, want 1", lines)
	}
	if !lastEntry.Applied {
		t.Error("audit entry Applied = false; must be true on success")
	}
	if lastEntry.Error != "" {
		t.Errorf("audit entry Error = %q; must be empty on success", lastEntry.Error)
	}
}

func TestToggleProxy_ApplyAppendsAuditOnFailure(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	tog := &fakeToggler{err: errors.New("api error")}
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, tog)

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	_, err := app.ToggleProxy(context.Background(), ctx, "example.com", "www", "A", false, true, t.TempDir(), auditPath)
	if err == nil {
		t.Fatal("expected error from failing toggler")
	}

	f, _ := os.Open(auditPath)
	defer f.Close()
	sc := bufio.NewScanner(f)
	var entry audit.Entry
	if sc.Scan() {
		_ = json.Unmarshal(sc.Bytes(), &entry)
	}
	if entry.Applied {
		t.Error("audit entry Applied = true on failure; must be false")
	}
	if entry.Error == "" {
		t.Error("audit entry Error empty on failure; must contain api error")
	}
}

func TestToggleProxy_AmbiguousMatchErrors(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	tog := &fakeToggler{}
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, tog)

	// Don't pass --type → both A and AAAA records named "www" match.
	_, err := app.ToggleProxy(context.Background(), ctx, "example.com", "www", "", false, false, t.TempDir(), filepath.Join(t.TempDir(), "audit.jsonl"))
	if !errors.Is(err, app.ErrAmbiguous) {
		t.Fatalf("err = %v, want ErrAmbiguous", err)
	}
	if tog.called {
		t.Error("Toggler called on ambiguous selector; must not")
	}
}

func TestToggleProxy_RecordNotFoundErrors(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, &fakeToggler{})

	_, err := app.ToggleProxy(context.Background(), ctx, "example.com", "missing", "A", false, false, t.TempDir(), filepath.Join(t.TempDir(), "audit.jsonl"))
	if !errors.Is(err, app.ErrRecordNotFound) {
		t.Errorf("err = %v, want ErrRecordNotFound", err)
	}
}

func TestToggleProxy_ZoneNotFoundErrors(t *testing.T) {
	zones, records := sampleZoneAndRecords()
	ctx := newAppCtx(&fakeListerForToggle{zones: zones, recordsByZone: records}, &fakeToggler{})

	_, err := app.ToggleProxy(context.Background(), ctx, "missing.com", "www", "A", false, false, t.TempDir(), filepath.Join(t.TempDir(), "audit.jsonl"))
	if !errors.Is(err, app.ErrZoneNotFound) {
		t.Errorf("err = %v, want ErrZoneNotFound", err)
	}
}
