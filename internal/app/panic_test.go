package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
)

func panicListerFixture() *fakeListerForToggle {
	return &fakeListerForToggle{
		zones: []domain.Zone{
			{ID: "z1", Name: "example.com", Status: "active"},
			{ID: "z2", Name: "other.com", Status: "active"},
		},
		recordsByZone: map[string][]domain.Record{
			"z1": {
				{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", Proxied: true, TTL: 300},
				{ID: "r2", Type: "A", Name: "api", Content: "5.6.7.8", Proxied: true, TTL: 300},
				{ID: "r3", Type: "TXT", Name: "_acme", Content: "challenge", Proxied: false, TTL: 300},
			},
			"z2": {
				{ID: "r4", Type: "A", Name: "www", Content: "9.9.9.9", Proxied: true, TTL: 300},
			},
		},
	}
}

func TestPanicPreviewZone_FiltersToProxiedRecordsOnly(t *testing.T) {
	ctx := newAppCtx(panicListerFixture(), &fakeToggler{})
	summary, err := app.PanicPreviewZone(context.Background(), ctx, "example.com")
	if err != nil {
		t.Fatalf("PanicPreviewZone: %v", err)
	}
	if got := len(summary.ProxiedRecords); got != 2 {
		t.Errorf("ProxiedRecords len = %d, want 2 (TXT _acme is proxied=false and should be excluded)", got)
	}
	for _, r := range summary.ProxiedRecords {
		if !r.Proxied {
			t.Errorf("preview included non-proxied record %s; must filter", r.Name)
		}
		if r.ZoneID != "z1" || r.ZoneName != "example.com" {
			t.Errorf("preview record missing zone enrichment: %+v", r)
		}
	}
}

func TestPanicPreviewZone_UnknownZoneErrors(t *testing.T) {
	ctx := newAppCtx(panicListerFixture(), &fakeToggler{})
	_, err := app.PanicPreviewZone(context.Background(), ctx, "missing.com")
	if !errors.Is(err, app.ErrPanicZoneNotFound) {
		t.Errorf("err = %v, want ErrPanicZoneNotFound", err)
	}
}

func TestPanicApplyZone_TogglesEveryProxiedRecordToFalse(t *testing.T) {
	tog := &captureToggler{}
	ctx := newAppCtx(panicListerFixture(), tog)

	summary, err := app.PanicPreviewZone(context.Background(), ctx, "example.com")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	stateDir := t.TempDir()
	snapDir := filepath.Join(stateDir, "snapshots")
	auditPath := filepath.Join(stateDir, "audit.jsonl")

	out, err := app.PanicApplyZone(context.Background(), ctx, summary, snapDir, auditPath)
	if err != nil {
		t.Fatalf("PanicApplyZone: %v", err)
	}

	if got := len(out.Results); got != 2 {
		t.Fatalf("Results len = %d, want 2", got)
	}
	for _, r := range out.Results {
		if !r.Applied {
			t.Errorf("result %s/%s not applied; want true (toggler is no-op fake)", r.Record.ZoneName, r.Record.Name)
		}
	}
	if got := len(tog.calls); got != 2 {
		t.Errorf("toggler call count = %d, want 2 (one per proxied record)", got)
	}
	for _, call := range tog.calls {
		if call.proxied != false {
			t.Errorf("toggler called with proxied=%v, want false (panic always un-proxies)", call.proxied)
		}
	}
}

func TestPanicApplyZone_PartialFailureContinuesAndReportsResults(t *testing.T) {
	// Toggler that errors on every other call so we exercise the partial-
	// failure path without making the test go-doc heavier than it has to.
	tog := &errOnSecondToggler{}
	ctx := newAppCtx(panicListerFixture(), tog)

	summary, err := app.PanicPreviewZone(context.Background(), ctx, "example.com")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	stateDir := t.TempDir()
	snapDir := filepath.Join(stateDir, "snapshots")
	auditPath := filepath.Join(stateDir, "audit.jsonl")

	out, err := app.PanicApplyZone(context.Background(), ctx, summary, snapDir, auditPath)
	// Partial failure does NOT bubble up an error (only every-record failure does).
	if err != nil {
		t.Fatalf("PanicApplyZone: %v", err)
	}

	var ok, fail int
	for _, r := range out.Results {
		if r.Err != nil {
			fail++
		} else if r.Applied {
			ok++
		}
	}
	if ok != 1 || fail != 1 {
		t.Errorf("got ok=%d fail=%d, want ok=1 fail=1", ok, fail)
	}
}

// errOnSecondToggler returns nil on the first call, an error on the second.
type errOnSecondToggler struct{ calls int }

func (e *errOnSecondToggler) SetProxied(_ context.Context, _ domain.Record, _ bool) error {
	e.calls++
	if e.calls == 2 {
		return errors.New("synthetic API failure")
	}
	return nil
}
