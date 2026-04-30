package cli_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/ui/cli"
)

type fakeListerForCLI struct {
	zones   []domain.Zone
	records map[string][]domain.Record
}

func (f *fakeListerForCLI) ListZones(_ context.Context) ([]domain.Zone, error) {
	return f.zones, nil
}
func (f *fakeListerForCLI) ListRecords(_ context.Context, zoneID string) ([]domain.Record, error) {
	return f.records[zoneID], nil
}

type recordingToggler struct {
	called bool
}

func (r *recordingToggler) SetProxied(_ context.Context, _ domain.Record, _ bool) error {
	r.called = true
	return nil
}

// stubVerifier satisfies domain.TokenVerifier for tests that traverse
// PersistentPreRunE. It always reports the token as active.
type stubVerifier struct{}

func (stubVerifier) Verify(_ context.Context) (domain.TokenStatus, error) {
	return domain.TokenStatus{Active: true}, nil
}

func buildToggleDeps() (cli.Dependencies, *recordingToggler) {
	tog := &recordingToggler{}
	build := func() (*app.Context, error) {
		return &app.Context{
			Logger:   slog.Default(),
			Version:  "0.0.0-test",
			Token:    "fake-token",
			Verifier: stubVerifier{},
			Lister: &fakeListerForCLI{
				zones: []domain.Zone{{ID: "z1", Name: "example.com", Status: "active"}},
				records: map[string][]domain.Record{
					"z1": {{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", Proxied: true, TTL: 300}},
				},
			},
			Toggler: tog,
		}, nil
	}
	return cli.Dependencies{Version: "0.0.0-test", Build: build}, tog
}

// TestToggleCmd_DryRunDoesNotInvokeToggler is the safety-critical test:
// without --apply the toggler must NEVER be called, no matter the diff.
func TestToggleCmd_DryRunDoesNotInvokeToggler(t *testing.T) {
	deps, tog := buildToggleDeps()
	root := cli.NewRootCmd(deps)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"toggle", "example.com/www", "--type=A", "--proxied=false"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if tog.called {
		t.Fatal("toggler called on dry-run path; safety violation")
	}

	out := buf.String()
	if !strings.Contains(out, "WOULD CHANGE") {
		t.Errorf("dry-run output missing 'WOULD CHANGE' marker; got: %q", out)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("dry-run output missing 'dry-run' hint; got: %q", out)
	}
}

func TestToggleCmd_BadSelectorErrors(t *testing.T) {
	deps, _ := buildToggleDeps()
	root := cli.NewRootCmd(deps)

	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"toggle", "noslashselector", "--proxied=false"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for bad selector, got nil")
	}
	if !strings.Contains(err.Error(), "<zone>/<name>") {
		t.Errorf("error message should explain selector shape; got: %v", err)
	}
}
