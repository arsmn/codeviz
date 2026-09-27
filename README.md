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
| `-min-score N` | Exit with status 1 if the overall score is below N (0–100). The output is still written. Other errors exit 2 |
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

### Overall score

A 0–100 rating, half **harmony** and half **health**, printed with its breakdown:

```
Score 86/100   harmony 84 · health 88
  harmony     84  deviation-weighted share of code outside the baseline: functions 5.2%, packages 8.9%
  complexity  81  4.5% of functions above cyclomatic 15
  size        84  5.8% of functions over 60 LOC
  coupling   100  packages import 1.4 others on average
  cohesion    93  mean cohesion 0.96 across 10 packages with ≥4 functions
```

- **Harmony** measures how well the code fits its *own* baseline. It uses the LOC-weighted deviation of functions (70%) and packages (30%).
- **Health** measures how that baseline compares with fixed norms. Without it, a uniformly tangled codebase would score as perfectly harmonious. Each component is linear between a "good" value (100) and a "bad" value (0):

| Component | Weight | 100 at | 0 at |
|---|---|---|---|
| complexity: share of functions with cyclomatic > 15 | 35% | ≤ 2% | ≥ 15% |
| size: share of functions over 60 LOC | 25% | ≤ 3% | ≥ 20% |
| coupling: mean internal imports per package | 20% | ≤ 3 | ≥ 12 |
| cohesion: mean over packages with ≥ 4 functions | 20% | 1.0 | ≤ 0.5 |

The thresholds are judgement calls, calibrated on the Go standard library,
where `net/http` scores 86, `cmd/go` 80 and `cmd/compile` 66. They are
constants in `internal/domain/score.go`.

In CI:

```sh
codeviz -q -format json -o codeviz.scene.json -min-score 75 .
```

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
| Medallion rim | codebase | The overall score. It is solid gold clockwise from the top for the score's share of the circle, and broken orange for the rest. Hover it for the breakdown. |
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
    "meanInstability": 0,    // mean Ce/(Ca+Ce)
    "score": {               // optional
      "total": 0, "harmony": 0, "health": 0,   // 0..100
      "components": [{ "name": "complexity", "group": "health", "value": 0, "weight": 0.35, "detail": "…" }]
    }
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
