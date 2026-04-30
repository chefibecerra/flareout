package app_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppLayerImportsAreClean asserts that every production Go file in
// internal/app/ imports nothing from internal/infra/ or internal/ui/.
//
// wiring.go is explicitly exempted by exact filename because it is the
// composition root — its sole purpose is to wire infra adapters into the
// app.Context. The exemption uses filepath.Base(path) == "wiring.go" to
// avoid accidentally exempting future files with similar path components.
//
// Test files (_test.go) are skipped: they are not part of the production
// dependency graph enforced by this layering invariant.
func TestAppLayerImportsAreClean(t *testing.T) {
	t.Helper()

	modPath := readModulePath(t)
	forbidden := []string{
		modPath + "/internal/infra/",
		modPath + "/internal/ui/",
	}

	// Tests run with the package directory as cwd, so "." is internal/app/.
	appDir := "."
	fset := token.NewFileSet()

	_ = filepath.WalkDir(appDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil // skip non-Go files and test files
		}
		// wiring.go is the composition root and is permitted to import infra.
		// Exemption is by exact filename — "rewiring.go" would NOT be exempt.
		if filepath.Base(path) == "wiring.go" {
			return nil
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
					t.Errorf("%s: forbidden import %q — app layer must not import infra or ui (only wiring.go is exempt)", path, p)
				}
			}
		}
		return nil
	})
}

// readModulePath reads the module directive from go.mod and returns the module
// path. The test runs from internal/app/, so go.mod is two levels up.
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
