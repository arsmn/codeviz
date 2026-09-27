package domain

import (
	"fmt"
	"testing"
)

func TestCohesion(t *testing.T) {
	connected := []Unit{
		{Name: "Service.Run", Refs: []string{"Service", "helper"}},
		{Name: "Service.Stop", Refs: []string{"Service"}},
		{Name: "helper"},
	}
	if got := cohesion(connected); got != 1 {
		t.Errorf("connected units: cohesion = %v, want 1", got)
	}
	bag := []Unit{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	if got := cohesion(bag); got != 0 {
		t.Errorf("unrelated units: cohesion = %v, want 0", got)
	}
}

func TestAnalyzeFlagsOutliers(t *testing.T) {
	var units []Unit
	for i := range 30 {
		units = append(units, Unit{Name: fmt.Sprint("f", i), LOC: 12 + i%5, Cyclomatic: 2 + i%3})
	}
	units = append(units, Unit{Name: "god", LOC: 400, Cyclomatic: 60})

	cb := Codebase{Packages: []Package{
		{ID: "a", LOC: 500, Units: units, Imports: []string{"b", "b", "a", "missing"}},
		{ID: "b", LOC: 300},
	}}
	an := Analyze(cb)

	a := an.Packages[0]
	if a.Efferent != 1 || an.Packages[1].Afferent != 1 {
		t.Fatalf("imports not normalized: efferent=%d afferent=%d", a.Efferent, an.Packages[1].Afferent)
	}
	for _, u := range a.Units {
		if u.Name == "god" {
			if u.Deviation < 0.5 || len(u.Reasons) != 2 {
				t.Errorf("god function: deviation=%.2f reasons=%v", u.Deviation, u.Reasons)
			}
		} else if u.Deviation != 0 {
			t.Errorf("%s: deviation %.2f, want 0", u.Name, u.Deviation)
		}
	}
}
