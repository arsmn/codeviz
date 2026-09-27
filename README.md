# CodeViz — architecture as generative art

Turns a codebase's structure and quality into a Persian-tile medallion built on a
hypotrochoid curve. Code that fits the codebase's own baseline renders as a
symmetric, harmonious tile; code that deviates breaks the pattern locally.

## CLI

```sh
go install ./cmd/codeviz        # or: go run ./cmd/codeviz …

codeviz [flags] [path]          # path defaults to "."
codeviz ./myproject -open       # write codeviz.html and open it
codeviz -o scene.json .         # just the scene JSON (format inferred from .json)
codeviz -o - -format json .     # to stdout
```

| Flag | |
|---|---|
| `-o` | Output file (default `codeviz.html`, or `codeviz.scene.json` with `-format json`), `-` for stdout |
| `-format` | `html` (standalone page, renderer embedded) or `json` |
| `-open` | Open the result in the browser |
| `-tests` | Include `_test.go` files (excluded by default) |
| `-generated` | Include `Code generated … DO NOT EDIT.` files (excluded by default) |
| `-top N` | How many most-deviant functions/packages to print (default 5) |
| `-q` | Quiet |

It finds the nearest `go.mod` above `path` to resolve imports, skips `vendor`,
`testdata`, hidden dirs and nested modules, and honours build constraints for
the current platform. It uses `go/parser`/`go/ast` only (no type checking),
so it works on code that doesn't compile.

### Architecture (hexagonal)

```
cmd/codeviz                  driving adapter: CLI
internal/app                 use cases: Analyze, Render
internal/ports               SourceParser (language in), Renderer (format out)
internal/domain              language-neutral model, metrics, baseline, deviation
internal/adapters/goparser   SourceParser for Go (go/ast)
internal/adapters/scene      Renderer → codeviz.scene/v0 JSON
internal/adapters/htmlpage   Renderer → standalone HTML (embeds web/medallion.html)
web                          the browser renderer + go:embed
```

A new language is a new `SourceParser` that emits `domain.Codebase`: packages,
imports, and functions with LOC, cyclomatic complexity and package-local
symbol references. Cohesion, coupling and scoring are shared.

### Metrics

- **Cyclomatic**: gocyclo rules. 1 + `if`, `for`, `range`, non-default `case`, `&&`, `||`. Closures count towards their parent.
- **Coupling**: afferent/efferent imports within the codebase, and instability = Ce/(Ca+Ce).
- **Cohesion**: 1 − (components−1)/(functions−1). Functions are connected when they reference each other or share a package-level symbol (e.g. a receiver type). This is syntactic, so it errs towards over-connecting.
- **Deviation**: a one-sided robust z-score against the codebase's own median/MAD (log scale for LOC and cyclomatic). Onset is at 2.5σ, and it saturates towards 1. Tunables are at the top of `internal/domain/analyze.go`.

## Renderer (`web/medallion.html`)

Single file, no build, no dependencies. Open it in a browser.

- **New codebase** generates a synthetic Go-like codebase with 1–3 planted anomalies.
  `?seed=N` in the URL pins a seed.
- **Load JSON…** (or drag a file onto the page) renders a `codeviz.scene/v0` file,
  e.g. `examples/tiny.scene.json`.
- Hover the curve or the border to see which function or package you're looking at, and why it deviates.

### Visual mapping

| Layer | Level | Driven by |
|---|---|---|
| Hypotrochoid curve | function | Each function owns a stretch of the curve (width ∝ √LOC). Its `deviation` jitters radius/phase/offset, quantizes the curve into jagged segments, turns it hot-coloured, and at extremes breaks it. |
| Baseline ghost (dotted gold) | codebase | The same curve with zero deviation: the shape the codebase "should" have. |
| `R` | codebase | Total LOC (overall scale). |
| `r` → lobe count | codebase | Median cyclomatic complexity (busier code, finer lobes). With *Lock symmetry*, `R/r` snaps to `p/q`, so the curve closes into an exact `p`-fold rosette. |
| `d` | codebase | Import density and mean instability (more coupling, loopier curve). |
| Border cartouches | package | One equal sector per package. The rosette's lobes come from mean cyclomatic complexity. `deviation` distorts the vine and rosette, dashes the outline, and shifts the colour. |
| Import lanes | package graph | Each import is an arc in its own concentric lane. Edges from deviant packages glow. |

Sectors are deliberately equal-sized. Only deviation breaks the symmetry, so a
big-but-normal package doesn't read as an anomaly.

## Scene contract — `codeviz.scene/v0` (draft)

The analyzer computes metrics, baseline and deviation, and the renderer only maps
them to visuals. Deviation lives in the domain, not the renderer.

```jsonc
{
  "schema": "codeviz.scene/v0",
  "project": "…", "language": "go",
  "summary": {
    "packages": 0, "units": 0, "totalLoc": 0,
    "medianLoc": 0, "medianCyclomatic": 0,
    "importDensity": 0,      // edges / (n·(n-1))
    "meanInstability": 0     // mean Ce/(Ca+Ce)
  },
  "packages": [{
    "id": "internal/app",
    "loc": 0, "afferent": 0, "efferent": 0, "instability": 0,
    "cohesion": 0,           // 0..1, higher = more cohesive
    "meanCyclomatic": 0,
    "imports": ["internal/domain"],   // intra-module only
    "deviation": 0,          // 0..1
    "reasons": ["imports 9 packages (4.1σ)"],
    "units": [{
      "id": "internal/app.(*Service).Run", "file": "service.go", "line": 12,
      "loc": 0, "cyclomatic": 0,
      "deviation": 0,        // 0..1
      "reasons": ["cyclomatic 34 is 9.8σ above baseline"]
    }]
  }]
}
```

Scores in the scene come from the Go domain. The page's synthetic generator
(`buildScene` in the HTML) has a rougher JavaScript stand-in, used only for
"New codebase".
