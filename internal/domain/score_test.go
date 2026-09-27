package domain

import (
	"fmt"
	"testing"
)

func TestOverallScore(t *testing.T) {
	clean := func(n, loc, cyc int) []Unit {
		var us []Unit
		for i := range n {
			us = append(us, Unit{Name: fmt.Sprint("f", i), LOC: loc + i%3, Cyclomatic: cyc, Refs: []string{"T"}})
		}
		return us
	}

	tidy := Analyze(Codebase{Packages: []Package{{ID: "a", LOC: 400, Units: clean(30, 12, 2)}}}).Summary.Score
	if tidy.Total < 99 || tidy.Harmony != 100 || tidy.Health < 99 {
		t.Errorf("uniform, simple codebase: %+v", tidy)
	}

	// Uniformly tangled: harmonious with itself, but unhealthy.
	tangled := Analyze(Codebase{Packages: []Package{{ID: "a", LOC: 9000, Units: clean(30, 300, 40)}}}).Summary.Score
	if tangled.Harmony != 100 || tangled.Health > 50 || tangled.Total > 75 {
		t.Errorf("uniformly tangled codebase: total=%.1f harmony=%.1f health=%.1f", tangled.Total, tangled.Harmony, tangled.Health)
	}

	// One outlier costs harmony.
	units := append(clean(30, 12, 2), Unit{Name: "god", LOC: 400, Cyclomatic: 60})
	outlier := Analyze(Codebase{Packages: []Package{{ID: "a", LOC: 800, Units: units}}}).Summary.Score
	if outlier.Harmony >= tidy.Harmony {
		t.Errorf("outlier should lower harmony: %.1f vs %.1f", outlier.Harmony, tidy.Harmony)
	}

	var sum float64
	for _, c := range outlier.Components {
		if c.Group == "health" {
			sum += c.Weight
		}
	}
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("health weights sum to %v", sum)
	}
}
