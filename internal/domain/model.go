// Package domain is the language-neutral core of codeviz: the facts a parser
// extracts from source code, and the baseline/deviation analysis built on them.
// It has no knowledge of any particular language or output format.
package domain

// Codebase is the set of structural facts a parser adapter extracts.
type Codebase struct {
	Name     string
	Language string
	Packages []Package
}

// Package is a module/package: the unit of the dependency graph.
type Package struct {
	ID              string   // path relative to the module root, e.g. "internal/app"
	Files           int      // number of source files
	LOC             int      // total source lines across files
	Imports         []string // IDs of other packages in this codebase
	ExternalImports int      // distinct imports from outside the codebase
	Units           []Unit
}

// Unit is a function or method.
type Unit struct {
	ID         string // globally unique, e.g. "internal/app.(*Service).Run"
	Name       string // package-local symbol, e.g. "Service.Run" or "NewService"
	File       string // path relative to the module root
	Line       int
	LOC        int
	Cyclomatic int
	Refs       []string // package-local symbols this unit references (see Name)
}

// Analysis is a Codebase measured against its own baseline.
type Analysis struct {
	Name     string
	Language string
	Summary  Summary
	Packages []PackageAnalysis
}

// Summary is the codebase-wide baseline profile.
type Summary struct {
	Packages         int
	Units            int
	TotalLOC         int
	MedianLOC        float64 // per unit
	MedianCyclomatic float64 // per unit
	ImportDensity    float64 // edges / (n·(n-1))
	MeanInstability  float64
	Score            OverallScore
}

// Score says how far something sits from the codebase baseline.
type Score struct {
	Deviation float64  // 0 = typical, → 1 = far outside the baseline
	Reasons   []string // human-readable, one per metric that deviates
}

type PackageAnalysis struct {
	ID              string
	Files           int
	LOC             int
	Imports         []string
	ExternalImports int
	Afferent        int     // packages in the codebase that import this one
	Efferent        int     // packages in the codebase this one imports
	Instability     float64 // Ce / (Ca + Ce)
	Cohesion        float64 // 0..1, see cohesion()
	MeanCyclomatic  float64
	Score
	Units []UnitAnalysis
}

type UnitAnalysis struct {
	Unit
	Score
}
