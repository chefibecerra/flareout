package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/domain"
	cfinfra "github.com/chefibecerra/flareout/internal/infra/cloudflare"
)

// Compile-time guard: *RecordToggler must satisfy domain.RecordToggler.
var _ domain.RecordToggler = (*cfinfra.RecordToggler)(nil)

func sampleRecordForToggle() domain.Record {
	return domain.Record{
		ID:       "rec-abc",
		Type:     "A",
		Name:     "www.example.com",
		Content:  "1.2.3.4",
		ZoneID:   "zone-xyz",
		ZoneName: "example.com",
		Proxied:  true,
		TTL:      300,
	}
}

// TestSetProxied_HappyPath stubs Cloudflare's Edit endpoint and confirms
// the adapter sends the expected body and zone path. The fakeToken must
// NOT appear in the request body.
func TestSetProxied_HappyPath(t *testing.T) {
	const fakeToken = "fake-token-for-testing"

	var capturedPath string
	var capturedBody map[string]any

	srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"rec-abc","type":"A","name":"www.example.com","content":"1.2.3.4","proxied":false,"ttl":300},"errors":[],"messages":[]}`))
	})

	tog, err := cfinfra.NewRecordToggler(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewRecordToggler: %v", err)
	}

	if err := tog.SetProxied(context.Background(), sampleRecordForToggle(), false); err != nil {
		t.Fatalf("SetProxied: %v", err)
	}

	// Path must include the zone ID and the record ID — protects against
	// missing cf.F() wrappers (lesson from cloudflare-go-v4-zones-dns-list discovery).
	if !strings.Contains(capturedPath, "zone-xyz") {
		t.Errorf("request path %q does not contain zone ID", capturedPath)
	}
	if !strings.Contains(capturedPath, "rec-abc") {
		t.Errorf("request path %q does not contain record ID", capturedPath)
	}

	// Body MUST contain the new proxied value.
	if got, ok := capturedBody["proxied"].(bool); !ok || got != false {
		t.Errorf("request body proxied = %v (ok=%v), want false", capturedBody["proxied"], ok)
	}
	// Body MUST preserve other record fields.
	if got, _ := capturedBody["name"].(string); got != "www.example.com" {
		t.Errorf("request body name = %q, want www.example.com", got)
	}
	if got, _ := capturedBody["type"].(string); got != "A" {
		t.Errorf("request body type = %q, want A", got)
	}
	if got, _ := capturedBody["content"].(string); got != "1.2.3.4" {
		t.Errorf("request body content = %q, want 1.2.3.4", got)
	}
}

func TestSetProxied_APIErrorReturnsWrapped(t *testing.T) {
	const fakeToken = "fake-token-for-testing"

	srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"messages":[],"result":null}`))
	})

	tog, err := cfinfra.NewRecordToggler(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewRecordToggler: %v", err)
	}

	err = tog.SetProxied(context.Background(), sampleRecordForToggle(), false)
	if err == nil {
		t.Fatal("SetProxied: expected error on 401, got nil")
	}
	if strings.Contains(err.Error(), fakeToken) {
		t.Errorf("error message contains raw token: %v", err)
	}
}

func TestSetProxied_ContextCancellationPropagates(t *testing.T) {
	const fakeToken = "fake-token-for-testing"

	srv := makeHandlerServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"result":{},"errors":[],"messages":[]}`))
	})

	tog, err := cfinfra.NewRecordToggler(fakeToken, cfinfra.WithBaseURL(srv.URL), cfinfra.WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewRecordToggler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = tog.SetProxied(ctx, sampleRecordForToggle(), false)
	if err == nil {
		t.Fatal("SetProxied: expected error on canceled context, got nil")
	}
}
