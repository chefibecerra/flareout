package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
)

// NewListCmd returns the "list" subcommand.
//
// list requires a verified token — no skip_verify annotation is set.
//
// Output mode is determined in order:
//  1. --json flag forces JSON regardless of terminal state.
//  2. cmd.OutOrStdout() is a non-TTY writer (buffer, pipe, redirect) → JSON.
//  3. cmd.OutOrStdout() is an *os.File with a TTY fd → TUI.
func NewListCmd(_ Dependencies) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Cloudflare DNS records and their proxy state",
		Long: "List all DNS records across every zone the API token can access. " +
			"Requires Zone:Read and DNS:Read token scopes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx, ok := AppCtxFrom(cmd.Context())
			if !ok {
				return fmt.Errorf("list: app.Context not in command context (PersistentPreRunE bypassed?)")
			}

			records, err := app.ListAllRecords(cmd.Context(), appCtx)
			if err != nil {
				return err
			}

			useJSON := asJSON || !stdoutIsTTY(cmd.OutOrStdout())
			if useJSON {
				return writeJSON(cmd.OutOrStdout(), records)
			}
			return runTUI(cmd.Context(), records, appCtx.Logger)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON instead of launching the interactive TUI")
	return cmd
}

// writeJSON encodes records as an indented JSON array to w.
// A nil slice is normalized to an empty slice so the output is always
// a JSON array ("[]"), never "null" (RL-11).
func writeJSON(w io.Writer, records []domain.Record) error {
	if records == nil {
		records = []domain.Record{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(records)
}

// stdoutIsTTY reports whether the writer is a TTY-backed *os.File.
// Returns false for bytes.Buffer, pipes, redirected stdout, or any non-file writer.
// This is intentionally conservative: non-file writers default to JSON mode,
// which is safe for automation and test harnesses (CL-04).
func stdoutIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}
