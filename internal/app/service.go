// Package app holds the codeviz use cases, wiring ports to the domain.
package app

import (
	"errors"
	"io"

	"github.com/arsmn/codeviz/internal/domain"
	"github.com/arsmn/codeviz/internal/ports"
)

var ErrNoPackages = errors.New("no source packages found")

type Service struct {
	Parser ports.SourceParser
}

// Analyze parses the tree at root and scores it against its own baseline.
func (s Service) Analyze(root string) (domain.Analysis, []string, error) {
	cb, warnings, err := s.Parser.Parse(root)
	if err != nil {
		return domain.Analysis{}, warnings, err
	}
	if len(cb.Packages) == 0 {
		return domain.Analysis{}, warnings, ErrNoPackages
	}
	return domain.Analyze(cb), warnings, nil
}

// Render analyzes the tree at root and writes it with r.
func (s Service) Render(root string, r ports.Renderer, w io.Writer) (domain.Analysis, []string, error) {
	a, warnings, err := s.Analyze(root)
	if err != nil {
		return a, warnings, err
	}
	return a, warnings, r.Render(w, a)
}
