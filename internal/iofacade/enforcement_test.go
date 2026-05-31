package iofacade_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestNoDirectIOOutsideFacades is the best-effort enforcement guard for issue
// #135. It parses every shipped .go file in the module and fails if any package
// other than the facades reaches the filesystem or network directly instead of
// going through internal/iofacade and internal/netfacade.
//
// This cannot statically prevent a determined bypass (a new import can always
// call os.* directly), but it catches accidental regressions and forces the
// facades to stay the single chokepoint for our own I/O. Paths handed to
// trusted third-party deps (SQLite driver, screp parser, scmapanalyzer) are
// opened inside those libraries and are out of scope here, as documented in the
// facade packages.
func TestNoDirectIOOutsideFacades(t *testing.T) {
	root := moduleRoot(t)

	// forbidden maps an import path to the set of disallowed selector names on
	// it. These are the file/network primitives that must go through a facade.
	forbidden := map[string]map[string]bool{
		"os": {
			"Open": true, "OpenFile": true, "Create": true, "CreateTemp": true,
			"ReadFile": true, "WriteFile": true, "Mkdir": true, "MkdirAll": true,
			"Remove": true, "RemoveAll": true, "ReadDir": true, "Rename": true,
			"Stat": true, "Lstat": true, "Chmod": true, "Symlink": true,
			"Link": true, "Readlink": true, "Truncate": true, "WriteString": true,
		},
		"io/ioutil": {
			"ReadFile": true, "WriteFile": true, "ReadDir": true, "ReadAll": true,
			"TempFile": true, "TempDir": true, "NopCloser": false,
		},
		"path/filepath": {
			"Walk": true, "WalkDir": true, "Glob": true,
		},
		"net/http": {
			"Get": true, "Post": true, "Head": true, "PostForm": true,
			"NewRequest": true, "NewRequestWithContext": true,
			"DefaultClient": true, "DefaultTransport": true,
		},
		"net": {
			"Dial": true, "DialTimeout": true, "DialIP": true, "DialTCP": true,
			"DialUDP": true, "DialUnix": true, "Dialer": true,
		},
	}

	var violations []string
	fset := token.NewFileSet()

	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDir(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if skipFile(root, path) {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}

		// Map local import name -> import path for this file.
		imports := map[string]string{}
		for _, imp := range file.Imports {
			impPath, _ := strconv.Unquote(imp.Path.Value)
			name := defaultPkgName(impPath)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			if name == "_" || name == "." {
				continue
			}
			imports[name] = impPath
		}

		rel, _ := filepath.Rel(root, path)
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			impPath, ok := imports[pkgIdent.Name]
			if !ok {
				return true
			}
			banned, ok := forbidden[impPath]
			if !ok {
				return true
			}
			if banned[sel.Sel.Name] {
				pos := fset.Position(sel.Pos())
				violations = append(violations, rel+":"+strconv.Itoa(pos.Line)+
					": "+pkgIdent.Name+"."+sel.Sel.Name+
					" — route through internal/iofacade or internal/netfacade")
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk module: %v", walkErr)
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("found %d direct I/O call(s) bypassing the facades (issue #135):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// skipDir excludes directories that are not part of the shipped binary or are
// the facades themselves (which legitimately call os/net directly).
func skipDir(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	switch rel {
	case ".git", ".claude", "node_modules", "dist",
		"internal/iofacade",           // the filesystem facade implementation
		"internal/netfacade",          // the network facade implementation
		"scripts",                     // dev-only debug scripts, not shipped
		"internal/dashboard/frontend", // React source, not Go
		"internal/dashboard/tools",    // build-time codegen, not shipped
		"internal/spec/tools":         // SPECIFICATION.md codegen, not shipped
		return true
	}
	base := filepath.Base(dir)
	return strings.HasPrefix(base, ".") && base != "." && base != ".."
}

// skipFile excludes individual files that are build-time tooling rather than
// part of the shipped binary.
func skipFile(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return strings.HasPrefix(rel, "scripts/") ||
		strings.HasPrefix(rel, "internal/dashboard/tools/") ||
		strings.HasPrefix(rel, "internal/spec/tools/")
}

func defaultPkgName(importPath string) string {
	// Good enough for the standard-library packages we gate on; their package
	// name matches the last path segment (os, http, filepath, net, ioutil).
	if i := strings.LastIndex(importPath, "/"); i >= 0 {
		return importPath[i+1:]
	}
	return importPath
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod above %s", dir)
		}
		dir = parent
	}
}
