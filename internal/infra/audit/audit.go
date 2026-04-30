// Package audit appends a JSONL trace of every proxy-toggle attempt
// (dry-run AND apply) to a local file. Future flareout-undo reads these
// entries to reconstruct what happened.
package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry is one audit log line.
//
// Fields are JSON-tagged with snake_case to match the wider project
// convention (matches the json tags on domain.Record). Timestamp is in
// RFC3339Nano so jq queries are straightforward.
type Entry struct {
	Timestamp     time.Time `json:"timestamp"`
	Zone          string    `json:"zone"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	BeforeProxied bool      `json:"before_proxied"`
	AfterProxied  bool      `json:"after_proxied"`
	Applied       bool      `json:"applied"`
	SnapshotPath  string    `json:"snapshot_path,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// Append writes entry as a single JSON line at the end of path. The parent
// directory is created with mode 0o700 if missing and the file with mode
// 0o600 on first write.
//
// Concurrent appends from a single process are safe because each call
// re-opens the file in O_APPEND mode and writes a single Write syscall;
// the OS-level append ordering is atomic for writes shorter than PIPE_BUF.
// Multi-process safety is NOT guaranteed — flareout is a single-user
// interactive tool, not a daemon.
func Append(entry Entry, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("audit: create dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("audit: open: %w", err)
	}

	line, err := json.Marshal(entry)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("audit: marshal: %w", err)
	}

	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("audit: write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("audit: close: %w", err)
	}
	return nil
}

// LastApplied scans the audit log file and returns the most recent entry
// with Applied=true. Returns (Entry{}, ErrNoAppliedEntry) if the file does
// not exist, is empty, or contains no applied entries.
//
// The scan reads the whole file (audit logs are small for an interactive
// tool — bounded by the number of toggles a single user makes). If the
// file grows large enough that this matters, switch to reverse-line-reading.
func LastApplied(path string) (Entry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, ErrNoAppliedEntry
		}
		return Entry{}, fmt.Errorf("audit: read: %w", err)
	}

	var last Entry
	var found bool
	for _, line := range splitLines(raw) {
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines silently — best-effort
		}
		if e.Applied {
			last = e
			found = true
		}
	}
	if !found {
		return Entry{}, ErrNoAppliedEntry
	}
	return last, nil
}

// ErrNoAppliedEntry indicates the audit log has no usable entry to undo.
var ErrNoAppliedEntry = errors.New("audit: no applied entry found")

func splitLines(raw []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			lines = append(lines, raw[start:i])
			start = i + 1
		}
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}
