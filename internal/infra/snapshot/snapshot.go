// Package snapshot persists pre-mutation state of DNS records to disk
// so that any failed apply or future undo flow has a recoverable copy of
// the record's last-known-good state.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chefibecerra/flareout/internal/domain"
)

// Now is the time source used when generating snapshot filenames. Tests
// override it to produce deterministic names; production leaves it at the
// default which uses the system clock.
var Now = func() time.Time { return time.Now().UTC() }

// Write persists rec as a JSON snapshot under dir. It returns the absolute
// path of the file. The directory is created with mode 0o700 if missing
// and the file is written with mode 0o600.
//
// Filename shape: <RFC3339-millisecond>-<safe-zone>-<safe-name>.json. The
// timestamp is at millisecond precision so concurrent writes from a single
// process do not collide for typical workloads.
func Write(rec domain.Record, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("snapshot: create dir: %w", err)
	}

	ts := Now().Format("20060102T150405.000Z")
	filename := fmt.Sprintf("%s-%s-%s.json", ts, sanitize(rec.ZoneName), sanitize(rec.Name))
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("snapshot: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("snapshot: write: %w", err)
	}

	return path, nil
}

// sanitize replaces filesystem-unsafe characters with underscores while
// preserving ASCII alphanumerics, dashes, and dots. Cross-platform safe
// (Windows forbids ':' and '*' for example).
func sanitize(s string) string {
	if s == "" {
		return "_"
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// Read parses a snapshot file at path and returns the recorded domain.Record.
// It is the inverse of Write — used by flareout-undo to recover the
// pre-mutation state of a record.
func Read(path string) (domain.Record, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return domain.Record{}, fmt.Errorf("snapshot: read: %w", err)
	}
	var rec domain.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return domain.Record{}, fmt.Errorf("snapshot: parse: %w", err)
	}
	return rec, nil
}
