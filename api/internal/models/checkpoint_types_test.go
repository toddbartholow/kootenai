package models

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// TestAllTriggerTypesCoversEveryConstant is the guard the validator test cannot
// be: the validator derives its accepted set from AllTriggerTypes, so a test
// iterating AllTriggerTypes can never notice a TriggerType constant that was
// declared but never added to the slice. That is the exact shape of the bug
// this list was introduced to fix -- seven types implemented in the evaluator
// but absent from the validator's accepted set -- restaged one level up.
func TestAllTriggerTypesCoversEveryConstant(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "checkpoint.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing checkpoint.go: %v", err)
	}

	declared := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		ident, ok := spec.Type.(*ast.Ident)
		if !ok || ident.Name != "TriggerType" {
			return true
		}
		for _, v := range spec.Values {
			lit, ok := v.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			value, err := strconv.Unquote(lit.Value)
			if err == nil {
				declared[value] = true
			}
		}
		return true
	})

	if len(declared) == 0 {
		t.Fatal("parsed no TriggerType constants; the guard is not working")
	}

	listed := map[string]bool{}
	for _, tt := range AllTriggerTypes {
		listed[string(tt)] = true
	}

	for value := range declared {
		if !listed[value] {
			t.Errorf("TriggerType %q is declared but missing from AllTriggerTypes, "+
				"so the template validator will reject it", value)
		}
	}
	for value := range listed {
		if !declared[value] {
			t.Errorf("AllTriggerTypes lists %q, which is not a declared TriggerType", value)
		}
	}
}
