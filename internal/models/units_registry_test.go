package models

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// TestGeometryRegistryComplete guards against adding a Unit*/Building* geometry
// var in units.go without also adding it to the registries in
// units_registry.go (which the SPECIFICATION.md generator enumerates). It parses
// units.go and counts every top-level `var X = Unit{...}` / `Building{...}`
// declaration, then asserts those counts match the registry lengths. Because the
// registries reference each var by identifier, a removed var won't compile and a
// renamed var is caught here — so the registries can't silently fall out of sync.
//
// This is the one place the spec machinery relies on AST parsing: Go has no way
// to enumerate package-level vars by type at runtime, so completeness can only
// be proven by reading the source.
func TestGeometryRegistryComplete(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current file")
	}
	unitsPath := filepath.Join(filepath.Dir(thisFile), "units.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, unitsPath, nil, 0)
	if err != nil {
		t.Fatalf("parse units.go: %v", err)
	}

	unitVars, buildingVars := 0, 0
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, val := range vs.Values {
				lit, ok := val.(*ast.CompositeLit)
				if !ok {
					continue
				}
				typeIdent, ok := lit.Type.(*ast.Ident)
				if !ok {
					continue
				}
				switch typeIdent.Name {
				case "Unit":
					unitVars++
				case "Building":
					buildingVars++
				}
			}
		}
	}

	if unitVars != len(unitGeometry) {
		t.Errorf("units.go declares %d Unit vars but unitGeometry has %d; add the missing var(s) to units_registry.go", unitVars, len(unitGeometry))
	}
	if buildingVars != len(buildingGeometry) {
		t.Errorf("units.go declares %d Building vars but buildingGeometry has %d; add the missing var(s) to units_registry.go", buildingVars, len(buildingGeometry))
	}
}
