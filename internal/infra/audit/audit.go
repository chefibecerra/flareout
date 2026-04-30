// Package audit appends a JSONL trace of every proxy-toggle attempt
// (dry-run AND apply) to a local file. Future flareout-undo reads these
// entries to reconstruct what happened.
package audit

import (
	"encoding/json"
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
