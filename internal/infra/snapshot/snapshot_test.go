package snapshot_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/infra/snapshot"
)

func sampleRecord() domain.Record {
	return domain.Record{
		ID:       "r1",
		Type:     "A",
		Name:     "www.example.com",
		Content:  "1.2.3.4",
		ZoneID:   "z1",
		ZoneName: "example.com",
		Proxied:  true,
		TTL:      300,
	}
}

func TestWrite_CreatesFileWith0600(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "snapshots")
	path, err := snapshot.Write(sampleRecord(), dir)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestWrite_CreatesDirectoryWith0700(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "snapshots")
	if _, err := snapshot.Write(sampleRecord(), dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %o, want 0700", info.Mode().Perm())
	}
}

func TestWrite_FileContainsRecordJSON(t *testing.T) {
	dir := t.TempDir()
	rec := sampleRecord()
	path, err := snapshot.Write(rec, dir)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	var got domain.Record
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if got != rec {
		t.Errorf("round-trip drift: got %+v, want %+v", got, rec)
	}
}

func TestWrite_FilenameFormat(t *testing.T) {
	// Pin the timestamp source so the filename is deterministic.
	original := snapshot.Now
	defer func() { snapshot.Now = original }()
	snapshot.Now = func() time.Time {
		return time.Date(2026, 4, 30, 12, 34, 56, 789_000_000, time.UTC)
	}

	dir := t.TempDir()
	path, err := snapshot.Write(sampleRecord(), dir)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	base := filepath.Base(path)
	if !strings.HasPrefix(base, "20260430T123456.789Z-") {
		t.Errorf("filename prefix = %q, want timestamp 20260430T123456.789Z prefix", base)
	}
	if !strings.HasSuffix(base, ".json") {
		t.Errorf("filename suffix = %q, want .json", base)
	}
	if !strings.Contains(base, "example.com") || !strings.Contains(base, "www.example.com") {
		t.Errorf("filename should embed zone+name; got %q", base)
	}
}

func TestWrite_SanitizesUnsafeFilenameSegments(t *testing.T) {
	dir := t.TempDir()
	rec := sampleRecord()
	rec.ZoneName = "weird zone/name"
	rec.Name = "*.wildcard"
	path, err := snapshot.Write(rec, dir)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	base := filepath.Base(path)
	for _, bad := range []string{"/", "\\", "*", " "} {
		if strings.Contains(base, bad) {
			t.Errorf("filename %q contains unsafe segment %q", base, bad)
		}
	}
}
