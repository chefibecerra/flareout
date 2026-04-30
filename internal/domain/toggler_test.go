package domain_test

import (
	"context"
	"testing"

	"github.com/chefibecerra/flareout/internal/domain"
)

// Compile-time guard: any inline type satisfying RecordToggler must keep
// the SetProxied method shape stable. If the port signature changes this
// fails to compile.
type stubToggler struct{ called bool }

func (s *stubToggler) SetProxied(_ context.Context, _ domain.Record, _ bool) error {
	s.called = true
	return nil
}

var _ domain.RecordToggler = (*stubToggler)(nil)

func TestDiff_IsNoOp(t *testing.T) {
	tests := []struct {
		name    string
		from    bool
		to      bool
		wantNop bool
	}{
		{"true to true", true, true, true},
		{"false to false", false, false, true},
		{"false to true", false, true, false},
		{"true to false", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := domain.Diff{
				Record:      domain.Record{ID: "r1", Name: "x.example.com"},
				FromProxied: tt.from,
				ToProxied:   tt.to,
			}
			if got := d.IsNoOp(); got != tt.wantNop {
				t.Errorf("IsNoOp() = %v, want %v", got, tt.wantNop)
			}
		})
	}
}

func TestDiff_PreservesRecord(t *testing.T) {
	r := domain.Record{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", Proxied: true, TTL: 300}
	d := domain.Diff{Record: r, FromProxied: true, ToProxied: false}
	if d.Record != r {
		t.Errorf("Diff.Record drifted from input: got %+v, want %+v", d.Record, r)
	}
}
