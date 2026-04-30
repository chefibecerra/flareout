package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/ui/cli"
)

// fakeVerifier is a test double for domain.TokenVerifier that always succeeds.
type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context) (domain.TokenStatus, error) {
	return domain.TokenStatus{Active: true}, nil
}

// fakeListLister is an inline test double for domain.RecordLister.
// Zones and records are provided at construction; all calls succeed unless
// zonesErr or recordsErr is set.
type fakeListLister struct {
	zones     []domain.Zone
	records   map[string][]domain.Record
	zonesErr  error
	recordErr error
}

func (f *fakeListLister) ListZones(_ context.Context) ([]domain.Zone, error) {
	if f.zonesErr != nil {
		return nil, f.zonesErr
	}
	return f.zones, nil
}

func (f *fakeListLister) ListRecords(_ context.Context, zoneID string) ([]domain.Record, error) {
	if f.recordErr != nil {
		return nil, f.recordErr
	}
	return f.records[zoneID], nil
}

// buildFakeAppCtx constructs a minimal *app.Context carrying the given lister.
// It intentionally bypasses app.Build so no env vars or network calls occur.
// Logger and Verifier are always set so PersistentPreRunE can complete without panic.
func buildFakeAppCtx(lister domain.RecordLister) *app.Context {
	return &app.Context{
		Lister:   lister,
		Verifier: fakeVerifier{},
		Logger:   slog.Default(),
	}
}

// execListJSON executes "flareout list --json" against a root command that
// goes through PersistentPreRunE (Build + VerifyTokenAtStartup + SetContext).
// Returns stdout contents (JSON) and the error from Execute.
// Stdout and stderr are kept separate to avoid mixing scope warnings with JSON.
func execListJSON(t *testing.T, appCtx *app.Context) ([]byte, error) {
	t.Helper()
	deps := cli.Dependencies{
		Version: "test",
		Build: func() (*app.Context, error) {
			return appCtx, nil
		},
	}
	root := cli.NewRootCmd(deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"list", "--json"})

	err := root.Execute()
	return stdout.Bytes(), err
}

// TestListCmd_JSONFlag_OutputsJSON confirms that --json produces a valid JSON
// array with the expected number of records and correct field values.
func TestListCmd_JSONFlag_OutputsJSON(t *testing.T) {
	lister := &fakeListLister{
		zones: []domain.Zone{
			{ID: "z1", Name: "alpha.example.com"},
			{ID: "z2", Name: "beta.example.com"},
		},
		records: map[string][]domain.Record{
			"z1": {
				{
					ID:       "r1",
					Type:     "A",
					Name:     "www.alpha.example.com",
					Content:  "1.2.3.4",
					ZoneID:   "z1",
					ZoneName: "alpha.example.com",
					Proxied:  true,
					TTL:      300,
				},
			},
			"z2": {
				{
					ID:       "r2",
					Type:     "AAAA",
					Name:     "www.beta.example.com",
					Content:  "::1",
					ZoneID:   "z2",
					ZoneName: "beta.example.com",
					Proxied:  false,
					TTL:      1,
				},
			},
		},
	}

	appCtx := buildFakeAppCtx(lister)
	out, err := execListJSON(t, appCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []domain.Record
	if jsonErr := json.Unmarshal(out, &got); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %q", jsonErr, string(out))
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records in JSON output, got %d", len(got))
	}
	// Sorted: alpha.example.com first.
	if got[0].ZoneName != "alpha.example.com" {
		t.Errorf("expected first record zone_name=alpha.example.com, got %q", got[0].ZoneName)
	}
	if got[0].Proxied != true {
		t.Errorf("expected first record proxied=true, got %v", got[0].Proxied)
	}
	if got[1].ZoneName != "beta.example.com" {
		t.Errorf("expected second record zone_name=beta.example.com, got %q", got[1].ZoneName)
	}
	if got[1].TTL != 1 {
		t.Errorf("expected second record ttl=1, got %d", got[1].TTL)
	}
}

// TestListCmd_JSON_NoTTY_AutoSelects verifies that without --json, when
// cmd.OutOrStdout() is a *bytes.Buffer (not an *os.File), the implementation
// treats it as non-TTY and falls back to JSON output automatically.
func TestListCmd_JSON_NoTTY_AutoSelects(t *testing.T) {
	lister := &fakeListLister{
		zones: []domain.Zone{
			{ID: "z1", Name: "gamma.example.com"},
		},
		records: map[string][]domain.Record{
			"z1": {
				{
					ID:       "r1",
					Type:     "A",
					Name:     "mail.gamma.example.com",
					Content:  "10.0.0.1",
					ZoneID:   "z1",
					ZoneName: "gamma.example.com",
					Proxied:  false,
					TTL:      300,
				},
			},
		},
	}

	appCtx := buildFakeAppCtx(lister)
	deps := cli.Dependencies{
		Version: "test",
		Build: func() (*app.Context, error) {
			return appCtx, nil
		},
	}
	root := cli.NewRootCmd(deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	// No --json flag; stdout redirected to buffer (non-TTY path).
	root.SetArgs([]string{"list"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v (stderr: %q)", err, stderr.String())
	}

	var got []domain.Record
	if jsonErr := json.Unmarshal(stdout.Bytes(), &got); jsonErr != nil {
		t.Fatalf("expected JSON auto-selected for non-TTY; output is not valid JSON: %v\nraw: %q",
			jsonErr, stdout.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record in JSON output, got %d", len(got))
	}
}

// TestListCmd_NoTokenFlag confirms that the list command does not register a
// --token flag at any level (CL-13 / CS-06 extension).
func TestListCmd_NoTokenFlag(t *testing.T) {
	deps := cli.Dependencies{
		Version: "test",
		Build: func() (*app.Context, error) {
			return &app.Context{Verifier: fakeVerifier{}, Logger: slog.Default()}, nil
		},
	}
	root := cli.NewRootCmd(deps)

	for _, sub := range root.Commands() {
		if sub.Name() != "list" {
			continue
		}
		if f := sub.Flags().Lookup("token"); f != nil {
			t.Errorf("found --token flag on list command (CS-06 violation)")
		}
		if f := sub.PersistentFlags().Lookup("token"); f != nil {
			t.Errorf("found --token persistent flag on list command (CS-06 violation)")
		}
		return
	}
	t.Error("list subcommand not found on root")
}

// TestListCmd_JSONFlagExists_DefaultsFalse verifies the --json flag is
// registered on list and defaults to false (CL-02).
func TestListCmd_JSONFlagExists_DefaultsFalse(t *testing.T) {
	deps := cli.Dependencies{
		Version: "test",
		Build: func() (*app.Context, error) {
			return &app.Context{Verifier: fakeVerifier{}, Logger: slog.Default()}, nil
		},
	}
	root := cli.NewRootCmd(deps)

	for _, sub := range root.Commands() {
		if sub.Name() == "list" {
			f := sub.Flags().Lookup("json")
			if f == nil {
				t.Fatal("expected --json flag on list command, not found")
			}
			if f.DefValue != "false" {
				t.Errorf("expected --json default to be false, got %q", f.DefValue)
			}
			if f.Value.Type() != "bool" {
				t.Errorf("expected --json to be bool flag, got type %q", f.Value.Type())
			}
			return
		}
	}
	t.Error("list subcommand not found on root")
}

// TestListCmd_EmptyZones_ReturnsEmptyArray confirms that when no zones
// exist, --json outputs an empty JSON array (RL-11).
func TestListCmd_EmptyZones_ReturnsEmptyArray(t *testing.T) {
	lister := &fakeListLister{
		zones:   []domain.Zone{},
		records: map[string][]domain.Record{},
	}
	appCtx := buildFakeAppCtx(lister)
	out, err := execListJSON(t, appCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output must be a JSON array, not null (RL-11).
	trimmed := bytes.TrimSpace(out)
	if !bytes.HasPrefix(trimmed, []byte("[")) {
		t.Errorf("expected JSON array output (starting with '['), got: %q", string(trimmed))
	}
	var got []domain.Record
	if jsonErr := json.Unmarshal(out, &got); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %q", jsonErr, string(out))
	}
	if len(got) != 0 {
		t.Errorf("expected 0 records for empty zones, got %d", len(got))
	}
}

// TestListCmd_APIError_ReturnsErr confirms that when ListZones returns an
// error, Execute returns a non-nil error (CL-08).
func TestListCmd_APIError_ReturnsErr(t *testing.T) {
	lister := &fakeListLister{
		zonesErr: errAPIDown,
	}
	appCtx := buildFakeAppCtx(lister)
	_, err := execListJSON(t, appCtx)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

// TestListCmd_NoSkipVerifyAnnotation confirms that list does NOT carry
// skip_verify annotation (CL-07).
func TestListCmd_NoSkipVerifyAnnotation(t *testing.T) {
	deps := cli.Dependencies{
		Version: "test",
		Build: func() (*app.Context, error) {
			return &app.Context{Verifier: fakeVerifier{}, Logger: slog.Default()}, nil
		},
	}
	root := cli.NewRootCmd(deps)
	for _, sub := range root.Commands() {
		if sub.Name() == "list" {
			if v, ok := sub.Annotations["skip_verify"]; ok && v == "true" {
				t.Error("list command must not have skip_verify=true annotation")
			}
			return
		}
	}
	t.Error("list subcommand not found on root")
}

// sentinel error used in multiple tests.
var errAPIDown = &apiError{msg: "API is down"}

type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }
