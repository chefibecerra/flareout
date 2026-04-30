package audit_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chefibecerra/flareout/internal/infra/audit"
)

func sampleEntry() audit.Entry {
	return audit.Entry{
		Timestamp:     time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
		Zone:          "example.com",
		Name:          "www",
		Type:          "A",
		BeforeProxied: true,
		AfterProxied:  false,
		Applied:       true,
		SnapshotPath:  "/tmp/snap.json",
	}
}

func TestAppend_CreatesParentDirectoryWith0700(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "dir", "audit.jsonl")

	if err := audit.Append(sampleEntry(), path); err != nil {
		t.Fatalf("Append: %v", err)
	}

	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %o, want 0700", info.Mode().Perm())
	}
}

func TestAppend_FileMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := audit.Append(sampleEntry(), path); err != nil {
		t.Fatalf("Append: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestAppend_RoundTripsEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	want := sampleEntry()
	if err := audit.Append(want, path); err != nil {
		t.Fatalf("Append: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Errorf("audit line must end with newline, got %q", raw)
	}

	var got audit.Entry
	if err := json.Unmarshal(raw[:len(raw)-1], &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Errorf("timestamp drift: got %v, want %v", got.Timestamp, want.Timestamp)
	}
	got.Timestamp = want.Timestamp // strip nano-precision noise across encode/decode
	if got != want {
		t.Errorf("entry drift: got %+v, want %+v", got, want)
	}
}

func TestAppend_AppendsAcrossMultipleCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	for i := 0; i < 3; i++ {
		entry := sampleEntry()
		entry.Name = "host" + string(rune('0'+i))
		if err := audit.Append(entry, path); err != nil {
			t.Fatalf("Append iter %d: %v", i, err)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var lines int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines++
		var e audit.Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Errorf("line %d not valid JSON: %v", lines, err)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if lines != 3 {
		t.Errorf("got %d lines, want 3", lines)
	}
}
