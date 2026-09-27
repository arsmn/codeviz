// Package scene is the Renderer adapter for the codeviz.scene/v0 JSON
// contract that the browser renderer consumes.
package scene

import (
	"encoding/json"
	"io"
	"math"

	"github.com/arsmn/codeviz/internal/domain"
)

const Schema = "codeviz.scene/v0"

type Scene struct {
	Schema   string    `json:"schema"`
	Project  string    `json:"project"`
	Language string    `json:"language"`
	Summary  Summary   `json:"summary"`
	Packages []Package `json:"packages"`
}

type Summary struct {
	Packages         int     `json:"packages"`
	Units            int     `json:"units"`
	TotalLoc         int     `json:"totalLoc"`
	MedianLoc        float64 `json:"medianLoc"`
	MedianCyclomatic float64 `json:"medianCyclomatic"`
	ImportDensity    float64 `json:"importDensity"`
	MeanInstability  float64 `json:"meanInstability"`
	Score            *Score  `json:"score,omitempty"` // optional in v0
}

// Score is the overall 0–100 rating: half harmony, half health.
type Score struct {
	Total      float64     `json:"total"`
	Harmony    float64     `json:"harmony"`
	Health     float64     `json:"health"`
	Components []Component `json:"components"`
}

type Component struct {
	Name   string  `json:"name"`
	Group  string  `json:"group"`
	Value  float64 `json:"value"`
	Weight float64 `json:"weight"`
	Detail string  `json:"detail"`
}

type Package struct {
	ID              string   `json:"id"`
	Files           int      `json:"files"`
	Loc             int      `json:"loc"`
	Afferent        int      `json:"afferent"`
	Efferent        int      `json:"efferent"`
	ExternalImports int      `json:"externalImports"`
	Instability     float64  `json:"instability"`
	Cohesion        float64  `json:"cohesion"`
	MeanCyclomatic  float64  `json:"meanCyclomatic"`
	Imports         []string `json:"imports"`
	Deviation       float64  `json:"deviation"`
	Reasons         []string `json:"reasons"`
	Units           []Unit   `json:"units"`
}

type Unit struct {
	ID         string   `json:"id"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Loc        int      `json:"loc"`
	Cyclomatic int      `json:"cyclomatic"`
	Deviation  float64  `json:"deviation"`
	Reasons    []string `json:"reasons"`
}

// From maps an analysis onto the scene contract.
func From(a domain.Analysis) Scene {
	s := Scene{
		Schema:   Schema,
		Project:  a.Name,
		Language: a.Language,
		Summary: Summary{
			Packages:         a.Summary.Packages,
			Units:            a.Summary.Units,
			TotalLoc:         a.Summary.TotalLOC,
			MedianLoc:        round(a.Summary.MedianLOC),
			MedianCyclomatic: round(a.Summary.MedianCyclomatic),
			ImportDensity:    round(a.Summary.ImportDensity),
			MeanInstability:  round(a.Summary.MeanInstability),
			Score:            fromScore(a.Summary.Score),
		},
		Packages: make([]Package, 0, len(a.Packages)),
	}
	for _, p := range a.Packages {
		sp := Package{
			ID:              p.ID,
			Files:           p.Files,
			Loc:             p.LOC,
			Afferent:        p.Afferent,
			Efferent:        p.Efferent,
			ExternalImports: p.ExternalImports,
			Instability:     round(p.Instability),
			Cohesion:        round(p.Cohesion),
			MeanCyclomatic:  round(p.MeanCyclomatic),
			Imports:         nonNil(p.Imports),
			Deviation:       round(p.Deviation),
			Reasons:         nonNil(p.Reasons),
			Units:           make([]Unit, 0, len(p.Units)),
		}
		for _, u := range p.Units {
			sp.Units = append(sp.Units, Unit{
				ID:         u.ID,
				File:       u.File,
				Line:       u.Line,
				Loc:        u.LOC,
				Cyclomatic: u.Cyclomatic,
				Deviation:  round(u.Deviation),
				Reasons:    nonNil(u.Reasons),
			})
		}
		s.Packages = append(s.Packages, sp)
	}
	return s
}

func fromScore(o domain.OverallScore) *Score {
	s := &Score{Total: round1(o.Total), Harmony: round1(o.Harmony), Health: round1(o.Health), Components: []Component{}}
	for _, c := range o.Components {
		s.Components = append(s.Components, Component{Name: c.Name, Group: c.Group, Value: round1(c.Value), Weight: c.Weight, Detail: c.Detail})
	}
	return s
}

// JSON renders the scene as indented JSON.
type JSON struct{}

func (JSON) Render(w io.Writer, a domain.Analysis) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(From(a))
}

func round(x float64) float64 { return math.Round(x*1e4) / 1e4 }

func round1(x float64) float64 { return math.Round(x*10) / 10 }

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
