package org_test

import (
	"reflect"
	"testing"

	"github.com/mattjoyce/shearline/examples/hrchat/org"
)

func TestGenerate_ShapeAndDeterminism(t *testing.T) {
	a, b := org.Generate(1), org.Generate(1)
	if n := len(a.Employees); n < 950 || n > 1050 {
		t.Fatalf("%d employees, want about a thousand", n)
	}
	if !reflect.DeepEqual(a.Facts(), b.Facts()) {
		t.Fatal("one seed gave two organisations")
	}
	if reflect.DeepEqual(a.Facts(), org.Generate(2).Facts()) {
		t.Fatal("two seeds gave one organisation")
	}
	roots := 0
	for _, e := range a.Employees {
		if e.ManagerID == "" {
			roots++
			continue
		}
		if _, ok := a.Get(e.ManagerID); !ok {
			t.Fatalf("%s reports to unknown %s", e.ID, e.ManagerID)
		}
		if len(a.Chain(e.ID)) == 0 || a.Chain(e.ID)[len(a.Chain(e.ID))-1] != a.Employees[0].ID {
			t.Fatalf("%s's chain does not end at the CEO: %v", e.ID, a.Chain(e.ID))
		}
	}
	if roots != 1 {
		t.Fatalf("%d roots", roots)
	}
}
