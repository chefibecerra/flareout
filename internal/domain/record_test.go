// Copyright 2026 JOSE MARIA BECERRA VAZQUEZ
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package domain_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/chefibecerra/flareout/internal/domain"
)

func TestZone(t *testing.T) {
	t.Run("zero value is valid and comparable", func(t *testing.T) {
		var z domain.Zone
		if z != (domain.Zone{}) {
			t.Error("zero-value Zone should equal empty Zone literal")
		}
		if z.ID != "" || z.Name != "" || z.Status != "" {
			t.Error("zero-value Zone should have all empty string fields")
		}
	})

	t.Run("field assignment round-trips", func(t *testing.T) {
		tests := []struct {
			name   string
			zone   domain.Zone
			wantID string
			wantNm string
			wantSt string
		}{
			{
				name:   "active zone",
				zone:   domain.Zone{ID: "zone-1", Name: "example.com", Status: "active"},
				wantID: "zone-1",
				wantNm: "example.com",
				wantSt: "active",
			},
			{
				name:   "pending zone",
				zone:   domain.Zone{ID: "zone-2", Name: "test.io", Status: "pending"},
				wantID: "zone-2",
				wantNm: "test.io",
				wantSt: "pending",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.zone.ID != tt.wantID {
					t.Errorf("ID: got %q, want %q", tt.zone.ID, tt.wantID)
				}
				if tt.zone.Name != tt.wantNm {
					t.Errorf("Name: got %q, want %q", tt.zone.Name, tt.wantNm)
				}
				if tt.zone.Status != tt.wantSt {
					t.Errorf("Status: got %q, want %q", tt.zone.Status, tt.wantSt)
				}
			})
		}
	})
}

func TestRecord(t *testing.T) {
	t.Run("zero value is valid and comparable", func(t *testing.T) {
		var r domain.Record
		if r != (domain.Record{}) {
			t.Error("zero-value Record should equal empty Record literal")
		}
		if r.ID != "" || r.Type != "" || r.Name != "" || r.Content != "" {
			t.Error("zero-value Record should have all empty string fields")
		}
		if r.ZoneID != "" || r.ZoneName != "" {
			t.Error("zero-value Record should have empty zone fields")
		}
		// Proxied is bool: zero value MUST be false, not nil.
		if r.Proxied {
			t.Error("zero-value Record.Proxied must be false")
		}
		if r.TTL != 0 {
			t.Error("zero-value Record.TTL must be 0")
		}
	})

	t.Run("field assignment round-trips", func(t *testing.T) {
		tests := []struct {
			name string
			rec  domain.Record
		}{
			{
				name: "proxied A record",
				rec: domain.Record{
					ID:       "rec-1",
					Type:     "A",
					Name:     "www.example.com",
					Content:  "1.2.3.4",
					ZoneID:   "zone-1",
					ZoneName: "example.com",
					Proxied:  true,
					TTL:      1,
				},
			},
			{
				name: "unproxied TXT record with empty content",
				rec: domain.Record{
					ID:       "rec-2",
					Type:     "TXT",
					Name:     "example.com",
					Content:  "",
					ZoneID:   "zone-1",
					ZoneName: "example.com",
					Proxied:  false,
					TTL:      300,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := tt.rec
				if r.ID != tt.rec.ID {
					t.Errorf("ID: got %q, want %q", r.ID, tt.rec.ID)
				}
				if r.Type != tt.rec.Type {
					t.Errorf("Type: got %q, want %q", r.Type, tt.rec.Type)
				}
				if r.Name != tt.rec.Name {
					t.Errorf("Name: got %q, want %q", r.Name, tt.rec.Name)
				}
				if r.Content != tt.rec.Content {
					t.Errorf("Content: got %q, want %q", r.Content, tt.rec.Content)
				}
				if r.ZoneID != tt.rec.ZoneID {
					t.Errorf("ZoneID: got %q, want %q", r.ZoneID, tt.rec.ZoneID)
				}
				if r.ZoneName != tt.rec.ZoneName {
					t.Errorf("ZoneName: got %q, want %q", r.ZoneName, tt.rec.ZoneName)
				}
				if r.Proxied != tt.rec.Proxied {
					t.Errorf("Proxied: got %v, want %v", r.Proxied, tt.rec.Proxied)
				}
				if r.TTL != tt.rec.TTL {
					t.Errorf("TTL: got %d, want %d", r.TTL, tt.rec.TTL)
				}
			})
		}
	})

	t.Run("Proxied is bool not pointer", func(t *testing.T) {
		// Proxied zero value is false (not nil). This documents the nil-normalization
		// contract: the infra adapter converts SDK *bool nil to false at the boundary,
		// so the domain type always holds a definite value.
		proxiedTrue := domain.Record{Proxied: true}
		if !proxiedTrue.Proxied {
			t.Error("Record{Proxied: true}.Proxied should be true")
		}
		var proxiedDefault domain.Record
		if proxiedDefault.Proxied {
			t.Error("default Record.Proxied should be false (zero value of bool)")
		}
	})

	t.Run("TTL is int64 with large value round-trip", func(t *testing.T) {
		const largeTTL int64 = 86400
		r := domain.Record{TTL: largeTTL}
		if r.TTL != largeTTL {
			t.Errorf("TTL: got %d, want %d", r.TTL, largeTTL)
		}
	})
}

func TestRecord_JSON(t *testing.T) {
	rec := domain.Record{
		ID:       "rec-abc",
		Type:     "A",
		Name:     "api.example.com",
		Content:  "203.0.113.1",
		ZoneID:   "zone-xyz",
		ZoneName: "example.com",
		Proxied:  true,
		TTL:      3600,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Assert exact JSON key names (snake_case as required by RL-10).
	requiredKeys := []string{
		`"id"`,
		`"type"`,
		`"name"`,
		`"content"`,
		`"zone_id"`,
		`"zone_name"`,
		`"proxied"`,
		`"ttl"`,
	}
	for _, key := range requiredKeys {
		if !bytes.Contains(data, []byte(key)) {
			t.Errorf("marshaled JSON missing required key %s; got: %s", key, data)
		}
	}

	// Round-trip: unmarshal back and compare field-by-field.
	var got domain.Record
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got != rec {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", got, rec)
	}

	// Proxied must survive round-trip as a boolean, never null (RL-10).
	if !bytes.Contains(data, []byte(`"proxied":true`)) {
		t.Errorf("proxied=true must marshal as boolean true, not null; got: %s", data)
	}

	// Empty Content round-trips cleanly (RL-13).
	emptyContent := domain.Record{Content: ""}
	emptyData, err := json.Marshal(emptyContent)
	if err != nil {
		t.Fatalf("json.Marshal empty Content: %v", err)
	}
	var gotEmpty domain.Record
	if err := json.Unmarshal(emptyData, &gotEmpty); err != nil {
		t.Fatalf("json.Unmarshal empty Content: %v", err)
	}
	if gotEmpty.Content != "" {
		t.Errorf("empty Content round-trip: got %q, want %q", gotEmpty.Content, "")
	}
}

// TestRecordLister_InterfaceShape is a compile-time check that RecordLister
// is a well-formed interface with the expected method signatures. If the
// interface changes in a breaking way, this test package will fail to compile.
func TestRecordLister_InterfaceShape(t *testing.T) {
	// Verify the interface methods are what we expect via a local implementation.
	type fakeRecordLister struct{}
	listZonesFn := func(ctx context.Context) ([]domain.Zone, error) { return nil, nil }
	listRecordsFn := func(ctx context.Context, zoneID string) ([]domain.Record, error) {
		return nil, nil
	}

	// Both functions must match the interface signatures exactly.
	var _ domain.RecordLister = fakeRecordListersImpl{
		listZonesFn:   listZonesFn,
		listRecordsFn: listRecordsFn,
	}
	_ = fakeRecordLister{} // prevent unused warning
}

// fakeRecordListersImpl is a minimal implementation of domain.RecordLister
// used exclusively in TestRecordLister_InterfaceShape to assert the interface
// shape compiles correctly.
type fakeRecordListersImpl struct {
	listZonesFn   func(ctx context.Context) ([]domain.Zone, error)
	listRecordsFn func(ctx context.Context, zoneID string) ([]domain.Record, error)
}

func (f fakeRecordListersImpl) ListZones(ctx context.Context) ([]domain.Zone, error) {
	return f.listZonesFn(ctx)
}

func (f fakeRecordListersImpl) ListRecords(ctx context.Context, zoneID string) ([]domain.Record, error) {
	return f.listRecordsFn(ctx, zoneID)
}
