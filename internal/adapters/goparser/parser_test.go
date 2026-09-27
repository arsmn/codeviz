package goparser

import (
	"testing"

	"github.com/arsmn/codeviz/internal/domain"
)

func TestParseSample(t *testing.T) {
	cb, warnings, err := Parser{}.Parse("testdata/sample")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if cb.Name != "example.com/sample" || cb.Language != "go" {
		t.Errorf("codebase = %q/%q", cb.Name, cb.Language)
	}
	if len(cb.Packages) != 2 {
		t.Fatalf("got %d packages, want 2: %+v", len(cb.Packages), cb.Packages)
	}
	units := map[string]domain.Unit{}
	for _, p := range cb.Packages {
		for _, u := range p.Units {
			units[u.ID] = u
		}
	}

	root, store := cb.Packages[0], cb.Packages[1]
	if root.ID != "sample" || store.ID != "internal/store" {
		t.Errorf("package IDs = %q, %q", root.ID, store.ID)
	}
	if len(root.Imports) != 1 || root.Imports[0] != "internal/store" || root.ExternalImports != 1 {
		t.Errorf("root imports = %v, external = %d", root.Imports, root.ExternalImports)
	}

	// for, if, &&, ||, case 0, case 1,2 → 1 + 6
	if got := units["sample.run"].Cyclomatic; got != 7 {
		t.Errorf("run cyclomatic = %d, want 7", got)
	}
	put, ok := units["internal/store.(*Store).Put"]
	if !ok {
		t.Fatalf("missing method unit; have %v", keys(units))
	}
	if put.Name != "Store.Put" || put.File != "internal/store/store.go" || put.Line != 7 {
		t.Errorf("Put = %+v", put)
	}
	for _, excluded := range []string{"sample.ignoredByBuildTag", "internal/store.helperForTests", "internal/store.Generated"} {
		if _, ok := units[excluded]; ok {
			t.Errorf("%s should be excluded", excluded)
		}
	}

	withTests, _, _ := Parser{IncludeTests: true, IncludeGenerated: true}.Parse("testdata/sample")
	if n := len(withTests.Packages[1].Units); n != 6 {
		t.Errorf("with tests+generated: store has %d units, want 6", n)
	}
}

func TestReferences(t *testing.T) {
	cb, _, _ := Parser{}.Parse("testdata/sample")
	for _, u := range cb.Packages[1].Units {
		switch u.Name {
		case "New":
			if len(u.Refs) != 1 || u.Refs[0] != "Store" {
				t.Errorf("New refs = %v", u.Refs)
			}
		case "unrelated":
			if len(u.Refs) != 0 {
				t.Errorf("unrelated refs = %v", u.Refs)
			}
		}
	}
}

func keys(m map[string]domain.Unit) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
