package catalog_test

import (
	"encoding/json"
	"testing"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
)

func TestCatalog(t *testing.T) {
	if err := catalog.Check(); err != nil {
		t.Fatal(err)
	}
	b, err := catalog.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Tools []struct{ Name string } }
	if err := json.Unmarshal(b, &m); err != nil || len(m.Tools) != len(catalog.Tools) {
		t.Fatalf("manifest: %v, %d tools", err, len(m.Tools))
	}
	used := map[string]bool{}
	for _, tool := range catalog.Tools {
		used[tool.Action] = true
	}
	for _, a := range catalog.ActionNames() {
		if !used[a] {
			t.Errorf("action %s has no tool", a)
		}
	}
}
