// Package ports defines the boundaries of the codeviz core: how source code
// comes in (SourceParser, one adapter per language) and how an analysis
// goes out (Renderer, one adapter per output format).
package ports

import (
	"io"

	"github.com/arsmn/codeviz/internal/domain"
)

// SourceParser extracts language-neutral structural facts from a source tree.
// Warnings report files or directories that were skipped but did not stop the parse.
type SourceParser interface {
	Parse(root string) (cb domain.Codebase, warnings []string, err error)
}

// Renderer writes an analysis in some output format.
type Renderer interface {
	Render(w io.Writer, a domain.Analysis) error
}
