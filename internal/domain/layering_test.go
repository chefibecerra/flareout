package domain_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDomainLayerImportsAreClean asserts that every production Go file in
// internal/domain/ imports nothing from internal/app, internal/infra, or
// internal/ui. Stdlib and third-party imports are allowed; only the
// module's own higher-level layers are forbidden.
func TestDomainLayerImportsAreClean(t *testing.T) {
	t.Helper()

	modPath := readModulePath(t)
	forbidden := []string{
		modPath + "/internal/app",
		modPath + "/internal/infra",
		modPath + "/internal/ui",
	}

	// Tests run with the package directory as cwd, so "." is internal/domain/.
	domainDir := "."
	fset := token.NewFileSet()

	_ = filepath.WalkDir(domainDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil // skip non-Go files and test files
		}
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Errorf("parse %s: %v", path, parseErr)
			return nil
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if strings.HasPrefix(p, bad) {
					t.Errorf("%s: forbidden import %q — domain must not import app/infra/ui", path, p)
				}
			}
		}
		return nil
	})
}

// readModulePath reads the module directive from go.mod and returns the module
// path. The test runs from internal/domain/, so go.mod is two levels up.
func readModulePath(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}
	t.Fatal("go.mod has no module directive")
	return ""
}
