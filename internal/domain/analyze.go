package domain

import (
	"fmt"
	"math"
	"slices"
	"sort"
)

// Deviation is a one-sided robust z-score (median/MAD) against the
// codebase's own distribution: only "too big", "too complex", "too coupled"
// and "too incoherent" count. z ≤ 2 is typical; beyond that it saturates
// towards 1. Real code has a long natural tail, so the onset sits at 2.5σ.
// Floors on the spread keep tiny or uniform codebases from flagging mild
// differences.
const (
	zThreshold = 2.5
	zSoftness  = 1.5

	floorLogLOC    = 0.35 // ≈ ×1.4 in size
	floorLogCyclo  = 0.30
	floorEfferent  = 1.0
	floorCohesion  = 0.25
	minCohesionN   = 4 // cohesion of 1–3 functions says little
	floorLogPkgLOC = 0.35
)

func deviationFromZ(z float64) float64 {
	return 1 - math.Exp(-math.Max(0, z-zThreshold)/zSoftness)
}

// Analyze computes coupling, cohesion, the codebase baseline, and a
// deviation score for every package and unit. Output order is deterministic.
func Analyze(cb Codebase) Analysis {
	pkgs := slices.Clone(cb.Packages)
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ID < pkgs[j].ID })

	index := make(map[string]int, len(pkgs))
	for i, p := range pkgs {
		index[p.ID] = i
	}

	out := make([]PackageAnalysis, len(pkgs))
	edges := 0
	for i, p := range pkgs {
		imports := normalizeImports(p.ID, p.Imports, index)
		edges += len(imports)
		units := make([]UnitAnalysis, len(p.Units))
		for k, u := range p.Units {
			units[k] = UnitAnalysis{Unit: u}
		}
		sort.Slice(units, func(a, b int) bool {
			if units[a].File != units[b].File {
				return units[a].File < units[b].File
			}
			return units[a].Line < units[b].Line
		})
		out[i] = PackageAnalysis{
			ID:              p.ID,
			Files:           p.Files,
			LOC:             p.LOC,
			Imports:         imports,
			ExternalImports: p.ExternalImports,
			Efferent:        len(imports),
			Cohesion:        cohesion(p.Units),
			Units:           units,
		}
	}
	for _, p := range out {
		for _, imp := range p.Imports {
			out[index[imp]].Afferent++
		}
	}

	var (
		unitLOC, unitCyc       []float64
		logUnitLOC, logUnitCyc []float64
		totalLOC               int
		sumInstability         float64
	)
	for i := range out {
		p := &out[i]
		if p.Afferent+p.Efferent > 0 {
			p.Instability = float64(p.Efferent) / float64(p.Afferent+p.Efferent)
		}
		sumInstability += p.Instability
		totalLOC += p.LOC
		sumCyc := 0
		for _, u := range p.Units {
			sumCyc += u.Cyclomatic
			unitLOC = append(unitLOC, float64(u.LOC))
			unitCyc = append(unitCyc, float64(u.Cyclomatic))
			logUnitLOC = append(logUnitLOC, logPos(u.LOC))
			logUnitCyc = append(logUnitCyc, logPos(u.Cyclomatic))
		}
		if len(p.Units) > 0 {
			p.MeanCyclomatic = float64(sumCyc) / float64(len(p.Units))
		}
	}

	summary := Summary{
		Packages:         len(out),
		Units:            len(unitLOC),
		TotalLOC:         totalLOC,
		MedianLOC:        median(unitLOC),
		MedianCyclomatic: median(unitCyc),
	}
	if n := len(out); n > 1 {
		summary.ImportDensity = float64(edges) / float64(n*(n-1))
	}
	if len(out) > 0 {
		summary.MeanInstability = sumInstability / float64(len(out))
	}

	scoreUnits(out, summary, robustOf(logUnitLOC, floorLogLOC), robustOf(logUnitCyc, floorLogCyclo))
	scorePackages(out)
	summary.Score = overallScore(out)

	return Analysis{Name: cb.Name, Language: cb.Language, Summary: summary, Packages: out}
}

func scoreUnits(pkgs []PackageAnalysis, s Summary, bLOC, bCyc robust) {
	for i := range pkgs {
		for k := range pkgs[i].Units {
			u := &pkgs[i].Units[k]
			zl := bLOC.z(logPos(u.LOC))
			zc := bCyc.z(logPos(u.Cyclomatic))
			if zl > zThreshold {
				u.Reasons = append(u.Reasons, fmt.Sprintf("%d LOC; codebase median is %g (%.1fσ)", u.LOC, s.MedianLOC, zl))
			}
			if zc > zThreshold {
				u.Reasons = append(u.Reasons, fmt.Sprintf("cyclomatic %d; codebase median is %g (%.1fσ)", u.Cyclomatic, s.MedianCyclomatic, zc))
			}
			u.Deviation = deviationFromZ(math.Max(zl, zc))
		}
	}
}

func scorePackages(pkgs []PackageAnalysis) {
	eff := make([]float64, len(pkgs))
	coh := make([]float64, len(pkgs))
	loc := make([]float64, len(pkgs))
	for i, p := range pkgs {
		eff[i], coh[i], loc[i] = float64(p.Efferent), p.Cohesion, logPos(p.LOC)
	}
	bEff, bCoh, bLOC := robustOf(eff, floorEfferent), robustOf(coh, floorCohesion), robustOf(loc, floorLogPkgLOC)

	for i := range pkgs {
		p := &pkgs[i]
		ze := bEff.z(float64(p.Efferent))
		zc := -bCoh.z(p.Cohesion) // low cohesion is the problem
		if len(p.Units) < minCohesionN {
			zc = 0
		}
		zl := bLOC.z(logPos(p.LOC))
		if ze > zThreshold {
			p.Reasons = append(p.Reasons, fmt.Sprintf("imports %d packages; median is %g (%.1fσ)", p.Efferent, bEff.m, ze))
		}
		if zc > zThreshold {
			p.Reasons = append(p.Reasons, fmt.Sprintf("cohesion %.2f; median is %.2f (%.1fσ)", p.Cohesion, bCoh.m, zc))
		}
		if zl > zThreshold {
			p.Reasons = append(p.Reasons, fmt.Sprintf("%d LOC (%.1fσ above typical package size)", p.LOC, zl))
		}
		p.Deviation = deviationFromZ(max(ze, zc, zl))
	}
}

// cohesion is 1 − (components−1)/(units−1), where units are connected when
// they reference each other or share a package-local symbol (e.g. a
// receiver type). One connected web of functions scores 1; a bag of
// unrelated helpers scores 0.
func cohesion(units []Unit) float64 {
	if len(units) <= 1 {
		return 1
	}
	uf := unionFind{}
	for _, u := range units {
		uf.find(u.Name)
		for _, r := range u.Refs {
			uf.union(u.Name, r)
		}
	}
	roots := map[string]bool{}
	for _, u := range units {
		roots[uf.find(u.Name)] = true
	}
	return 1 - float64(len(roots)-1)/float64(len(units)-1)
}

func normalizeImports(self string, imports []string, index map[string]int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, imp := range imports {
		if _, ok := index[imp]; ok && imp != self && !seen[imp] {
			seen[imp] = true
			out = append(out, imp)
		}
	}
	sort.Strings(out)
	return out
}

func logPos(n int) float64 { return math.Log(math.Max(1, float64(n))) }

type robust struct{ m, s float64 }

func (r robust) z(x float64) float64 { return (x - r.m) / r.s }

func robustOf(xs []float64, floor float64) robust {
	m := median(xs)
	dev := make([]float64, len(xs))
	for i, x := range xs {
		dev[i] = math.Abs(x - m)
	}
	return robust{m: m, s: math.Max(1.4826*median(dev), floor)}
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

type unionFind map[string]string

func (u unionFind) find(x string) string {
	p, ok := u[x]
	if !ok {
		u[x] = x
		return x
	}
	if p == x {
		return x
	}
	root := u.find(p)
	u[x] = root
	return root
}

func (u unionFind) union(a, b string) { u[u.find(a)] = u.find(b) }
