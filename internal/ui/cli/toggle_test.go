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

// --- flareout undo CLI tests ---

// TestUndoCmd_NoAuditEntryPrintsNothing confirms the undo subcommand
// surfaces "nothing to undo" cleanly when the audit log is missing.
func TestUndoCmd_NoAuditEntryPrintsNothing(t *testing.T) {
	deps, _ := buildToggleDeps()
	root := cli.NewRootCmd(deps)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"undo"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "nothing to undo") {
		t.Errorf("expected 'nothing to undo' in output; got: %q", out)
	}
}

// TestUndoCmd_RevertsAfterPriorApply seeds a real audit + snapshot via
// app.ApplyToggle, then runs the undo subcommand and asserts the toggler
// was called a second time with the reverted value.
func TestUndoCmd_RevertsAfterPriorApply(t *testing.T) {
	// State dir override: the undo command resolves snapshot+audit from
	// logging.StateLogPath which uses XDG_STATE_HOME. Point it at TempDir
	// so the test does not pollute or read from the user's real state.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	deps, tog := buildToggleDeps()
	root := cli.NewRootCmd(deps)

	// Seed: drive a toggle through the CLI in dry-run-then-apply so the
	// subsequent undo has something real to revert.
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"toggle", "example.com/www", "--type=A", "--proxied=false", "--apply"})
	if err := root.Execute(); err != nil {
		t.Fatalf("seed apply: %v", err)
	}
	if !tog.called {
		t.Fatal("seed: toggler was not called; setup failed")
	}

	// Reset for the undo execution.
	tog.called = false
	root2 := cli.NewRootCmd(deps)
	var buf bytes.Buffer
	root2.SetOut(&buf)
	root2.SetErr(&buf)
	root2.SetArgs([]string{"undo"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("undo Execute: %v", err)
	}

	if !tog.called {
		t.Error("undo did not call toggler")
	}
	if !strings.Contains(buf.String(), "REVERTED") {
		t.Errorf("undo output missing 'REVERTED' marker; got: %q", buf.String())
	}
}

// --- flareout panic CLI tests ---

// TestPanicCmd_RequiresZoneFlag confirms the subcommand errors cleanly
// when --zone is omitted (the only required flag).
func TestPanicCmd_RequiresZoneFlag(t *testing.T) {
	deps, _ := buildToggleDeps()
	root := cli.NewRootCmd(deps)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"panic"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --zone is missing, got nil")
	}
	if !strings.Contains(err.Error(), "--zone") {
		t.Errorf("error should mention --zone; got: %v", err)
	}
}

// TestPanicCmd_YesFlagSkipsConfirmation_AppliesAllProxied verifies the
// --yes path: each currently-proxied record gets a toggler call with
// proxied=false. The fixture has one proxied record (www) so we expect
// exactly one call.
func TestPanicCmd_YesFlagSkipsConfirmation_AppliesAllProxied(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	deps, tog := buildToggleDeps()
	root := cli.NewRootCmd(deps)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"panic", "--zone=example.com", "--yes"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !tog.called {
		t.Error("panic --yes did not invoke toggler")
	}
	if !strings.Contains(buf.String(), "PANIC SUMMARY") {
		t.Errorf("output missing 'PANIC SUMMARY'; got: %q", buf.String())
	}
}

// TestPanicCmd_ConfirmationMismatchAborts confirms that piping the wrong
// answer to the confirmation prompt aborts without calling the toggler.
func TestPanicCmd_ConfirmationMismatchAborts(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	deps, tog := buildToggleDeps()
	root := cli.NewRootCmd(deps)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("not-the-zone-name\n"))
	root.SetArgs([]string{"panic", "--zone=example.com"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if tog.called {
		t.Error("panic invoked toggler despite confirmation mismatch")
	}
	if !strings.Contains(buf.String(), "aborted") {
		t.Errorf("output missing 'aborted'; got: %q", buf.String())
	}
}

// TestPanicCmd_NoProxiedRecordsExitsCleanly handles the case where the
// zone exists but has no proxied records to un-proxy.
func TestPanicCmd_NoProxiedRecordsExitsCleanly(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

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
					// All records non-proxied → panic should be a no-op
					"z1": {{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", Proxied: false, TTL: 300}},
				},
			},
			Toggler: tog,
		}, nil
	}
	deps := cli.Dependencies{Version: "0.0.0-test", Build: build}
	root := cli.NewRootCmd(deps)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"panic", "--zone=example.com", "--yes"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if tog.called {
		t.Error("panic invoked toggler when there were no proxied records to flip")
	}
	if !strings.Contains(buf.String(), "no proxied records") {
		t.Errorf("output missing 'no proxied records' message; got: %q", buf.String())
	}
}
