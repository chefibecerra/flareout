package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/ui/cli"
)

// panicBuild panics if invoked — used to confirm skip_verify prevents
// the lazy app.Context construction (and the token check that follows it)
// for the version command.
func panicBuild() (*app.Context, error) {
	panic("Build called when it should have been skipped")
}

func fakeDeps() cli.Dependencies {
	return cli.Dependencies{
		Version: "0.0.0-test",
		Build:   panicBuild,
	}
}

// TestRootHasVersionSubcmd asserts that "version" is registered as a subcommand
// of the root command.
func TestRootHasVersionSubcmd(t *testing.T) {
	root := cli.NewRootCmd(fakeDeps())

	found := false
	for _, cmd := range root.Commands() {
		if cmd.Name() == "version" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected 'version' subcommand to be registered on root, but it was not found")
	}
}

// TestVersionRunsWithoutVerifierAndPrintsVersion confirms that:
//
//	(a) panicBuild is NOT called (skip_verify annotation works end-to-end),
//	(b) the output contains "flareout 0.0.0-test",
//	(c) Execute returns nil.
func TestVersionRunsWithoutVerifierAndPrintsVersion(t *testing.T) {
	root := cli.NewRootCmd(fakeDeps())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})

	// If panicBuild fires this will panic — which the test runner converts
	// to a fatal failure, giving a clear signal that skip_verify broke.
	err := root.Execute()
	if err != nil {
		t.Fatalf("expected nil error from 'flareout version', got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "flareout 0.0.0-test") {
		t.Fatalf("expected output to contain %q, got: %q", "flareout 0.0.0-test", out)
	}
}

// TestNoTokenFlagExists walks the root command and all subcommands to assert
// that no flag named "token" exists at any level of the command tree (CS-06).
func TestNoTokenFlagExists(t *testing.T) {
	root := cli.NewRootCmd(fakeDeps())

	// Check root persistent flags.
	if f := root.PersistentFlags().Lookup("token"); f != nil {
		t.Errorf("found --token flag on root persistent flags (CS-06 violation)")
	}
	if f := root.Flags().Lookup("token"); f != nil {
		t.Errorf("found --token flag on root local flags (CS-06 violation)")
	}

	// Check all subcommands.
	for _, sub := range root.Commands() {
		if f := sub.Flags().Lookup("token"); f != nil {
			t.Errorf("found --token flag on subcommand %q (CS-06 violation)", sub.Name())
		}
		if f := sub.PersistentFlags().Lookup("token"); f != nil {
			t.Errorf("found --token flag on subcommand %q persistent flags (CS-06 violation)", sub.Name())
		}
	}
}
