package domain

import (
	"fmt"
	"math"
)

// The overall score is half harmony (how well the code fits its own
// baseline) and half health (how that baseline compares with widely used
// norms). Harmony alone would rate a uniformly tangled codebase as perfect.
//
// Each health component is linear between a "good" value (100) and a "bad"
// value (0). Thresholds were calibrated against the Go standard library.
const (
	harmonyWeight = 0.5

	harmonyGain       = 2.5 // deviation-weighted share of code → points lost
	harmonyUnitWeight = 0.7 // functions vs packages within harmony

	complexityLimit = 15 // cyclomatic; golangci-lint's gocyclo default
	complexityGood  = 0.02
	complexityBad   = 0.15

	sizeLimit = 60 // LOC per function
	sizeGood  = 0.03
	sizeBad   = 0.20

	couplingGood = 3.0 // mean internal imports per package
	couplingBad  = 12.0

	cohesionGood = 1.0
	cohesionBad  = 0.5
)

var healthWeights = map[string]float64{
	"complexity": 0.35,
	"size":       0.25,
	"coupling":   0.20,
	"cohesion":   0.20,
}

// OverallScore rates a whole codebase from 0 to 100.
type OverallScore struct {
	Total      float64
	Harmony    float64
	Health     float64
	Components []ScoreComponent
}

type ScoreComponent struct {
	Name   string  // harmony, complexity, size, coupling, cohesion
	Group  string  // "harmony" or "health"
	Value  float64 // 0..100
	Weight float64 // within its group
	Detail string
}

func overallScore(pkgs []PackageAnalysis) OverallScore {
	var (
		units, unitLOC, pkgLOC int
		unitDev, pkgDev        float64
		complex, long          int
		fanOut                 int
		cohSum                 float64
		cohN                   int
	)
	for _, p := range pkgs {
		pkgLOC += p.LOC
		pkgDev += p.Deviation * float64(p.LOC)
		fanOut += p.Efferent
		if len(p.Units) >= minCohesionN {
			cohSum += p.Cohesion
			cohN++
		}
		for _, u := range p.Units {
			units++
			unitLOC += u.LOC
			unitDev += u.Deviation * float64(u.LOC)
			if u.Cyclomatic > complexityLimit {
				complex++
			}
			if u.LOC > sizeLimit {
				long++
			}
		}
	}

	unitShare, pkgShare := ratio(unitDev, float64(unitLOC)), ratio(pkgDev, float64(pkgLOC))
	harmony := 100 * clamp01(1-harmonyGain*(harmonyUnitWeight*unitShare+(1-harmonyUnitWeight)*pkgShare))
	comps := []ScoreComponent{{
		Name: "harmony", Group: "harmony", Value: harmony, Weight: 1,
		Detail: fmt.Sprintf("deviation-weighted share of code outside the baseline: functions %.1f%%, packages %.1f%%", 100*unitShare, 100*pkgShare),
	}}

	complexShare, longShare := ratio(float64(complex), float64(units)), ratio(float64(long), float64(units))
	meanFanOut := ratio(float64(fanOut), float64(len(pkgs)))
	comps = append(comps,
		health("complexity", linear(complexShare, complexityGood, complexityBad),
			fmt.Sprintf("%.1f%% of functions above cyclomatic %d", 100*complexShare, complexityLimit)),
		health("size", linear(longShare, sizeGood, sizeBad),
			fmt.Sprintf("%.1f%% of functions over %d LOC", 100*longShare, sizeLimit)),
		health("coupling", linear(meanFanOut, couplingGood, couplingBad),
			fmt.Sprintf("packages import %.1f others on average", meanFanOut)),
	)
	if cohN > 0 {
		mean := cohSum / float64(cohN)
		comps = append(comps, health("cohesion", linear(mean, cohesionGood, cohesionBad),
			fmt.Sprintf("mean cohesion %.2f across %d packages with ≥%d functions", mean, cohN, minCohesionN)))
	} else {
		comps = append(comps, health("cohesion", 100, fmt.Sprintf("no package has ≥%d functions", minCohesionN)))
	}

	var healthScore float64
	for _, c := range comps {
		if c.Group == "health" {
			healthScore += c.Weight * c.Value
		}
	}
	return OverallScore{
		Total:      harmonyWeight*harmony + (1-harmonyWeight)*healthScore,
		Harmony:    harmony,
		Health:     healthScore,
		Components: comps,
	}
}

func health(name string, value float64, detail string) ScoreComponent {
	return ScoreComponent{Name: name, Group: "health", Value: value, Weight: healthWeights[name], Detail: detail}
}

// linear maps good → 100 and bad → 0, clamped; works in either direction.
func linear(x, good, bad float64) float64 {
	return 100 * clamp01((bad-x)/(bad-good))
}

func ratio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }
