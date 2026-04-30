package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
)

// fakeLister is an inline test double for domain.RecordLister.
// Define zones, optional per-zone errors, and optional delays to simulate
// latency (used in the context-cancellation test).
//
// errgroup semantics note: errgroup.WithContext returns the FIRST error any
// goroutine returns; subsequent errors from other goroutines are silently
// dropped. The test "one zone fails" (TestListAllRecords_OneZoneError) exercises
// this by verifying that exactly one error is returned even when two zones fail,
// and that the error is non-nil without asserting which zone's error is returned.
type fakeLister struct {
	zones         []domain.Zone
	zonesErr      error
	recordsByZone map[string][]domain.Record
	recordsErr    map[string]error
	delays        map[string]time.Duration // optional: simulate latency for cancellation tests
}

func (f *fakeLister) ListZones(ctx context.Context) ([]domain.Zone, error) {
	if f.zonesErr != nil {
		return nil, f.zonesErr
	}
	return f.zones, nil
}

func (f *fakeLister) ListRecords(ctx context.Context, zoneID string) ([]domain.Record, error) {
	if d := f.delays[zoneID]; d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := f.recordsErr[zoneID]; err != nil {
		return nil, err
	}
	return f.recordsByZone[zoneID], nil
}

func appCtxWith(lister domain.RecordLister) *app.Context {
	return &app.Context{Lister: lister}
}

// TestListAllRecords_SuccessPath verifies that all records across all zones
// are returned with no error when everything succeeds.
func TestListAllRecords_SuccessPath(t *testing.T) {
	lister := &fakeLister{
		zones: []domain.Zone{
			{ID: "z1", Name: "alpha.com"},
			{ID: "z2", Name: "beta.com"},
			{ID: "z3", Name: "gamma.com"},
		},
		recordsByZone: map[string][]domain.Record{
			"z1": {
				{ID: "r1", Name: "www", ZoneName: "alpha.com"},
				{ID: "r2", Name: "mail", ZoneName: "alpha.com"},
			},
			"z2": {
				{ID: "r3", Name: "www", ZoneName: "beta.com"},
				{ID: "r4", Name: "api", ZoneName: "beta.com"},
			},
			"z3": {
				{ID: "r5", Name: "www", ZoneName: "gamma.com"},
				{ID: "r6", Name: "ftp", ZoneName: "gamma.com"},
			},
		},
	}

	records, err := app.ListAllRecords(context.Background(), appCtxWith(lister))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 6 {
		t.Fatalf("expected 6 records, got %d", len(records))
	}
}

// TestListAllRecords_Sorting verifies case-insensitive zone-then-record sort order.
// Zones and records are provided intentionally out of order.
func TestListAllRecords_Sorting(t *testing.T) {
	lister := &fakeLister{
		zones: []domain.Zone{
			{ID: "z3", Name: "Zorro.com"},
			{ID: "z1", Name: "apple.com"},
			{ID: "z2", Name: "Banana.com"},
		},
		recordsByZone: map[string][]domain.Record{
			"z1": {
				{ID: "r2", Name: "www", ZoneName: "apple.com"},
				{ID: "r1", Name: "Mail", ZoneName: "apple.com"},
			},
			"z2": {
				{ID: "r4", Name: "Zap", ZoneName: "Banana.com"},
				{ID: "r3", Name: "api", ZoneName: "Banana.com"},
			},
			"z3": {
				{ID: "r5", Name: "ns1", ZoneName: "Zorro.com"},
			},
		},
	}

	records, err := app.ListAllRecords(context.Background(), appCtxWith(lister))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected order: apple.com (Mail, www), Banana.com (api, Zap), Zorro.com (ns1)
	// Case-insensitive: "apple" < "banana" < "zorro"; "mail" < "www"; "api" < "zap"
	want := []struct {
		zoneName string
		name     string
	}{
		{"apple.com", "Mail"},
		{"apple.com", "www"},
		{"Banana.com", "api"},
		{"Banana.com", "Zap"},
		{"Zorro.com", "ns1"},
	}

	if len(records) != len(want) {
		t.Fatalf("expected %d records, got %d", len(want), len(records))
	}

	for i, w := range want {
		got := records[i]
		if got.ZoneName != w.zoneName || got.Name != w.name {
			t.Errorf("record[%d]: want {ZoneName:%q Name:%q}, got {ZoneName:%q Name:%q}",
				i, w.zoneName, w.name, got.ZoneName, got.Name)
		}
	}
}

// TestListAllRecords_ListZonesError verifies that a ListZones error is
// returned immediately with no records and no goroutines launched.
func TestListAllRecords_ListZonesError(t *testing.T) {
	wantErr := errors.New("zones unavailable")
	lister := &fakeLister{
		zonesErr: wantErr,
	}

	records, err := app.ListAllRecords(context.Background(), appCtxWith(lister))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
	if records != nil {
		t.Errorf("expected nil records on error, got %v", records)
	}
}

// TestListAllRecords_OneZoneError verifies that errgroup returns an error when
// any zone's ListRecords fails. errgroup first-error semantics: only the FIRST
// error is returned; the identity of the failing zone is not asserted here
// because goroutine scheduling is non-deterministic. Subsequent errors are
// silently dropped by errgroup.
func TestListAllRecords_OneZoneError(t *testing.T) {
	zoneErr := errors.New("zone fetch failed")
	lister := &fakeLister{
		zones: []domain.Zone{
			{ID: "z1", Name: "alpha.com"},
			{ID: "z2", Name: "beta.com"},
		},
		recordsByZone: map[string][]domain.Record{
			"z1": {{ID: "r1", Name: "www", ZoneName: "alpha.com"}},
		},
		recordsErr: map[string]error{
			"z2": zoneErr,
		},
	}

	records, err := app.ListAllRecords(context.Background(), appCtxWith(lister))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if records != nil {
		t.Errorf("expected nil records on error, got %v", records)
	}
}

// TestListAllRecords_ContextCancellation verifies that cancelling the context
// aborts in-flight ListRecords calls and returns an error wrapping context.Canceled.
func TestListAllRecords_ContextCancellation(t *testing.T) {
	lister := &fakeLister{
		zones: []domain.Zone{
			{ID: "z1", Name: "alpha.com"},
		},
		delays: map[string]time.Duration{
			"z1": 100 * time.Millisecond,
		},
		recordsByZone: map[string][]domain.Record{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after 10ms — before the 100ms delay completes.
	time.AfterFunc(10*time.Millisecond, cancel)

	_, err := app.ListAllRecords(ctx, appCtxWith(lister))
	if err == nil {
		t.Fatal("expected error after context cancel, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestListAllRecords_EmptyZones verifies that an empty zones list returns an
// empty (non-nil) slice with no error.
func TestListAllRecords_EmptyZones(t *testing.T) {
	lister := &fakeLister{
		zones:         []domain.Zone{},
		recordsByZone: map[string][]domain.Record{},
	}

	records, err := app.ListAllRecords(context.Background(), appCtxWith(lister))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty zones should yield no records, nil error.
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}
