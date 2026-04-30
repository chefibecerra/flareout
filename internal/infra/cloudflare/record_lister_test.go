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

// Package cloudflare_test contains integration-style tests for the cloudflare infra adapters.
// CA-09: this package imports only internal/domain (no internal/app, no internal/ui).
package cloudflare_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/domain"
	cfinfra "github.com/chefibecerra/flareout/internal/infra/cloudflare"
)

// Compile-time assertion: *RecordLister must satisfy domain.RecordLister (CA-02).
// This line fails to compile if ListZones or ListRecords signatures change.
var _ domain.RecordLister = (*cfinfra.RecordLister)(nil)

// makeHandlerServer returns a test server driven by an http.HandlerFunc.
// Callers own the server lifetime; use defer server.Close() or t.Cleanup.
func makeHandlerServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	// Wrap the user-provided handler so requests for page>=2 return an empty
	// result. The cloudflare-go/v4 auto-pager (V4PagePaginationAutoPager.GetNextPage)
	// does NOT honor result_info.total_pages — it loops until Result.Items is empty.
	// Without this guard, stub handlers returning the same body on every call
	// cause infinite pagination and tests hang.
	wrapped := func(w http.ResponseWriter, r *http.Request) {
		if page := r.URL.Query().Get("page"); page != "" && page != "1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"result":[],"result_info":{"page":2,"per_page":20,"total_pages":1,"count":0,"total_count":0},"errors":[],"messages":[]}`))
			return
		}
		handler(w, r)
	}
	srv := httptest.NewServer(http.HandlerFunc(wrapped))
	t.Cleanup(srv.Close)
	return srv
}

// zonesEnvelope returns a minimal Cloudflare v4 JSON envelope for the zones response.
func zonesEnvelope(resultJSON string) string {
	return `{"success":true,"result":` + resultJSON +
		`,"result_info":{"page":1,"per_page":20,"total_pages":1,"count":1,"total_count":1}` +
		`,"errors":[],"messages":[]}`
}

// recordsEnvelope returns a minimal Cloudflare v4 JSON envelope for the DNS records response.
func recordsEnvelope(resultJSON string, count int) string {
	countStr := strconv.Itoa(count)
	return `{"success":true,"result":` + resultJSON +
		`,"result_info":{"page":1,"per_page":20,"total_pages":1,"count":` + countStr + `,"total_count":` + countStr + `}` +
		`,"errors":[],"messages":[]}`
}

// TestRecordLister_Constructor verifies CA-01: constructor signature mirrors NewTokenVerifier.
func TestRecordLister_Constructor(t *testing.T) {
	t.Run("NewRecordLister accepts token and optional WithBaseURL", func(t *testing.T) {
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(zonesEnvelope(`[]`)))
		})

		lister, err := cfinfra.NewRecordLister("fake-token", cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: unexpected error: %v", err)
		}
		if lister == nil {
			t.Fatal("NewRecordLister returned nil lister")
		}
	})

	t.Run("NewRecordLister with no options does not error", func(t *testing.T) {
		lister, err := cfinfra.NewRecordLister("any-token")
		if err != nil {
			t.Fatalf("NewRecordLister(noOpts): unexpected error: %v", err)
		}
		if lister == nil {
			t.Fatal("NewRecordLister returned nil lister with no options")
		}
	})
}

// TestRecordLister_ListZones covers CA-03, CA-08, CA-11.
func TestRecordLister_ListZones(t *testing.T) {
	const fakeToken = "fake-token-for-testing"

	t.Run("returns mapped zones from stub response (CA-03)", func(t *testing.T) {
		body := zonesEnvelope(`[{"id":"z1","name":"example.com","status":"active"}]`)
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		zones, err := lister.ListZones(context.Background())
		if err != nil {
			t.Fatalf("ListZones: unexpected error: %v", err)
		}
		if len(zones) != 1 {
			t.Fatalf("ListZones: got %d zones, want 1", len(zones))
		}
		if zones[0].ID != "z1" {
			t.Errorf("zones[0].ID = %q, want %q", zones[0].ID, "z1")
		}
		if zones[0].Name != "example.com" {
			t.Errorf("zones[0].Name = %q, want %q", zones[0].Name, "example.com")
		}
		if zones[0].Status != "active" {
			t.Errorf("zones[0].Status = %q, want %q", zones[0].Status, "active")
		}
	})

	t.Run("authorization header contains token (CA-11)", func(t *testing.T) {
		const testToken = "test-token-abc"
		var capturedAuth string
		body := zonesEnvelope(`[]`)
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			capturedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		})

		lister, err := cfinfra.NewRecordLister(testToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		_, _ = lister.ListZones(context.Background())
		if !strings.Contains(capturedAuth, testToken) {
			t.Errorf("Authorization header %q does not contain token", capturedAuth)
		}
	})

	t.Run("server returns 401 — error returned, no panic, token not in message (CA-08)", func(t *testing.T) {
		body := `{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"messages":[],"result":null}`
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(body))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		_, err = lister.ListZones(context.Background())
		if err == nil {
			t.Fatal("ListZones: expected error on 401, got nil")
		}
		if strings.Contains(err.Error(), fakeToken) {
			t.Errorf("ListZones error message contains raw token: %v", err)
		}
	})

	t.Run("closed server returns ErrVerifyNetwork (CA-08)", func(t *testing.T) {
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {})
		// Close immediately to simulate network failure; t.Cleanup will run again but Close is idempotent.
		srv.Close()

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		_, err = lister.ListZones(context.Background())
		if err == nil {
			t.Fatal("ListZones: expected error after server closed, got nil")
		}
		if !errors.Is(err, domain.ErrVerifyNetwork) {
			t.Errorf("ListZones: error does not wrap ErrVerifyNetwork; got: %v", err)
		}
	})

	t.Run("context canceled before call propagates context.Canceled (CA-07)", func(t *testing.T) {
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(zonesEnvelope(`[]`)))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel before any call

		_, err = lister.ListZones(ctx)
		if err == nil {
			t.Fatal("ListZones: expected error on canceled context, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("ListZones: expected context.Canceled, got: %v", err)
		}
	})

	t.Run("empty zones response returns empty slice, no error (CA-03)", func(t *testing.T) {
		body := zonesEnvelope(`[]`)
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		zones, err := lister.ListZones(context.Background())
		if err != nil {
			t.Fatalf("ListZones: unexpected error: %v", err)
		}
		if len(zones) != 0 {
			t.Errorf("ListZones: got %d zones, want 0", len(zones))
		}
	})
}

// TestRecordLister_ListRecords covers CA-04, CA-05, CA-06, CA-07, CA-10.
func TestRecordLister_ListRecords(t *testing.T) {
	const fakeToken = "fake-token-for-testing"

	t.Run("maps record fields correctly (CA-05)", func(t *testing.T) {
		body := recordsEnvelope(`[{"id":"rec1","type":"A","name":"www.example.com","content":"1.2.3.4","zone_id":"abc123","zone_name":"example.com","proxied":true,"ttl":1}]`, 1)
		var capturedPath string
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		records, err := lister.ListRecords(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("ListRecords: unexpected error: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("ListRecords: got %d records, want 1", len(records))
		}
		r := records[0]
		if r.ID != "rec1" {
			t.Errorf("Record.ID = %q, want %q", r.ID, "rec1")
		}
		if r.Type != "A" {
			t.Errorf("Record.Type = %q, want %q", r.Type, "A")
		}
		if r.Name != "www.example.com" {
			t.Errorf("Record.Name = %q, want %q", r.Name, "www.example.com")
		}
		if r.Content != "1.2.3.4" {
			t.Errorf("Record.Content = %q, want %q", r.Content, "1.2.3.4")
		}
		if r.ZoneID != "abc123" {
			t.Errorf("Record.ZoneID = %q, want %q", r.ZoneID, "abc123")
		}
		// ZoneName is intentionally empty at this layer: cloudflare-go/v4
		// dns.RecordResponse does not carry zone_name, so the adapter leaves it
		// empty and the application layer enriches it from the Zone in hand.
		if r.ZoneName != "" {
			t.Errorf("Record.ZoneName = %q, want %q (adapter must not set ZoneName)", r.ZoneName, "")
		}
		if !r.Proxied {
			t.Errorf("Record.Proxied = false, want true")
		}
		if r.TTL != 1 {
			t.Errorf("Record.TTL = %d, want 1", r.TTL)
		}
		// CA-04: request URL must include zone_id in the path (z round-trip)
		if !strings.Contains(capturedPath, "abc123") {
			t.Errorf("request path %q does not contain zone ID %q", capturedPath, "abc123")
		}
	})

	t.Run("proxied null normalizes to false (CA-06)", func(t *testing.T) {
		// Three records: proxied true, proxied false, proxied null.
		body := recordsEnvelope(`[
			{"id":"r1","type":"A","name":"a.example.com","content":"1.1.1.1","zone_id":"z1","zone_name":"example.com","proxied":true,"ttl":1},
			{"id":"r2","type":"A","name":"b.example.com","content":"2.2.2.2","zone_id":"z1","zone_name":"example.com","proxied":false,"ttl":1},
			{"id":"r3","type":"NS","name":"example.com","content":"ns1.example.com","zone_id":"z1","zone_name":"example.com","proxied":null,"ttl":3600}
		]`, 3)
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		records, err := lister.ListRecords(context.Background(), "z1")
		if err != nil {
			t.Fatalf("ListRecords: unexpected error: %v", err)
		}
		if len(records) != 3 {
			t.Fatalf("ListRecords: got %d records, want 3", len(records))
		}

		// Find by ID for deterministic assertions.
		byID := make(map[string]domain.Record)
		for _, rec := range records {
			byID[rec.ID] = rec
		}

		if !byID["r1"].Proxied {
			t.Errorf("r1.Proxied: got false, want true (proxied:true in JSON)")
		}
		if byID["r2"].Proxied {
			t.Errorf("r2.Proxied: got true, want false (proxied:false in JSON)")
		}
		if byID["r3"].Proxied {
			t.Errorf("r3.Proxied: got true, want false (proxied:null in JSON — must normalize to false at adapter boundary)")
		}
	})

	t.Run("zone_id appears in request path — cf.F round-trip (CA-04)", func(t *testing.T) {
		body := recordsEnvelope(`[]`, 0)
		var capturedPath string
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		_, err = lister.ListRecords(context.Background(), "zone-xyz-789")
		if err != nil {
			t.Fatalf("ListRecords: unexpected error: %v", err)
		}
		if !strings.Contains(capturedPath, "zone-xyz-789") {
			t.Errorf("request path %q does not contain zoneID %q — cf.F(zoneID) may be missing", capturedPath, "zone-xyz-789")
		}
	})

	t.Run("context canceled before call propagates context.Canceled, not domain sentinel (CA-07)", func(t *testing.T) {
		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(recordsEnvelope(`[]`, 0)))
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel before call

		_, err = lister.ListRecords(ctx, "z1")
		if err == nil {
			t.Fatal("ListRecords: expected error on canceled context, got nil")
		}
		// Context errors MUST NOT be wrapped as domain sentinels (they propagate unwrapped).
		if !errors.Is(err, context.Canceled) {
			t.Errorf("ListRecords: expected context.Canceled, got: %v", err)
		}
	})

	t.Run("multi-record two-zone independent pagination (CA-10)", func(t *testing.T) {
		// Zone "z1" has 3 records; zone "z2" has 2 records.
		// Each call to ListRecords is independent — this test calls them sequentially.
		zone1Body := recordsEnvelope(`[
			{"id":"z1r1","type":"A","name":"a.z1.com","content":"1.1.1.1","zone_id":"z1","zone_name":"z1.com","proxied":false,"ttl":1},
			{"id":"z1r2","type":"A","name":"b.z1.com","content":"2.2.2.2","zone_id":"z1","zone_name":"z1.com","proxied":false,"ttl":1},
			{"id":"z1r3","type":"A","name":"c.z1.com","content":"3.3.3.3","zone_id":"z1","zone_name":"z1.com","proxied":false,"ttl":1}
		]`, 3)
		zone2Body := recordsEnvelope(`[
			{"id":"z2r1","type":"A","name":"a.z2.com","content":"4.4.4.4","zone_id":"z2","zone_name":"z2.com","proxied":false,"ttl":1},
			{"id":"z2r2","type":"A","name":"b.z2.com","content":"5.5.5.5","zone_id":"z2","zone_name":"z2.com","proxied":false,"ttl":1}
		]`, 2)

		srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "z1") {
				_, _ = w.Write([]byte(zone1Body))
			} else {
				_, _ = w.Write([]byte(zone2Body))
			}
		})

		lister, err := cfinfra.NewRecordLister(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
		if err != nil {
			t.Fatalf("NewRecordLister: %v", err)
		}

		z1Records, err := lister.ListRecords(context.Background(), "z1")
		if err != nil {
			t.Fatalf("ListRecords(z1): %v", err)
		}
		if len(z1Records) != 3 {
			t.Errorf("ListRecords(z1): got %d records, want 3", len(z1Records))
		}

		z2Records, err := lister.ListRecords(context.Background(), "z2")
		if err != nil {
			t.Fatalf("ListRecords(z2): %v", err)
		}
		if len(z2Records) != 2 {
			t.Errorf("ListRecords(z2): got %d records, want 2", len(z2Records))
		}

		// No cross-contamination.
		for _, r := range z1Records {
			if r.ZoneID != "z1" {
				t.Errorf("z1Records contains record with ZoneID=%q", r.ZoneID)
			}
		}
		for _, r := range z2Records {
			if r.ZoneID != "z2" {
				t.Errorf("z2Records contains record with ZoneID=%q", r.ZoneID)
			}
		}
	})
}
