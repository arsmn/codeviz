// Command codeviz renders a Go codebase's architecture as a Persian-tile
// medallion, where code that deviates from the codebase's own baseline
// visibly breaks the pattern.
//
//	codeviz [flags] [path]
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/arsmn/codeviz/internal/adapters/goparser"
	"github.com/arsmn/codeviz/internal/adapters/htmlpage"
	"github.com/arsmn/codeviz/internal/adapters/scene"
	"github.com/arsmn/codeviz/internal/app"
	"github.com/arsmn/codeviz/internal/domain"
	"github.com/arsmn/codeviz/internal/ports"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "codeviz:", err)
		}
		os.Exit(2)
	}
}

type options struct {
	out       string
	format    string
	tests     bool
	generated bool
	open      bool
	top       int
	quiet     bool
}

func run(args []string, stdout, stderr io.Writer) error {
	var o options
	fs := flag.NewFlagSet("codeviz", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.out, "o", "", `output file; "-" for stdout (default "codeviz.html", or "codeviz.scene.json" with -format json)`)
	fs.StringVar(&o.format, "format", "", "html or json (default: from the -o extension, else html)")
	fs.BoolVar(&o.tests, "tests", false, "include _test.go files")
	fs.BoolVar(&o.generated, "generated", false, `include generated files ("Code generated ... DO NOT EDIT.")`)
	fs.BoolVar(&o.open, "open", false, "open the result in a browser")
	fs.IntVar(&o.top, "top", 5, "number of most-deviant functions and packages to list")
	fs.BoolVar(&o.quiet, "q", false, "print nothing but errors")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: codeviz [flags] [path]\n\nRenders the Go code under path (default \".\") as a medallion.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	// Accept flags before and after the path.
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(positional) > 1 {
		fs.Usage()
		return fmt.Errorf("expected one path, got %d", len(positional))
	}
	root := "."
	if len(positional) == 1 {
		root = positional[0]
	}

	renderer, err := pickRenderer(&o)
	if err != nil {
		return err
	}

	svc := app.Service{Parser: goparser.Parser{IncludeTests: o.tests, IncludeGenerated: o.generated}}
	a, warnings, err := svc.Analyze(root)
	if !o.quiet {
		for _, w := range warnings {
			fmt.Fprintln(stderr, "warning:", w)
		}
	}
	if errors.Is(err, app.ErrNoPackages) {
		return fmt.Errorf("no Go packages found under %s", root)
	}
	if err != nil {
		return err
	}

	if err := write(o.out, stdout, func(w io.Writer) error { return renderer.Render(w, a) }); err != nil {
		return err
	}
	if !o.quiet {
		report(stderr, a, o)
	}
	if o.open && o.out != "-" {
		return openBrowser(o.out)
	}
	return nil
}

func pickRenderer(o *options) (ports.Renderer, error) {
	if o.format == "" {
		o.format = "html"
		if strings.EqualFold(filepath.Ext(o.out), ".json") {
			o.format = "json"
		}
	}
	switch o.format {
	case "html":
		if o.out == "" {
			o.out = "codeviz.html"
		}
		return htmlpage.Page{}, nil
	case "json":
		if o.out == "" {
			o.out = "codeviz.scene.json"
		}
		return scene.JSON{}, nil
	}
	return nil, fmt.Errorf("unknown -format %q (want html or json)", o.format)
}

func write(path string, stdout io.Writer, render func(io.Writer) error) error {
	if path == "-" {
		return render(stdout)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	if err := render(bw); err != nil {
		f.Close()
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func report(w io.Writer, a domain.Analysis, o options) {
	s := a.Summary
	dest := o.out
	if dest == "-" {
		dest = "stdout"
	}
	fmt.Fprintf(w, "%s: %d packages, %d functions, %d LOC (median function: %g LOC, cyclomatic %g) → %s\n",
		a.Name, s.Packages, s.Units, s.TotalLOC, s.MedianLOC, s.MedianCyclomatic, dest)
	if o.top <= 0 {
		return
	}

	type item struct {
		label   string
		score   domain.Score
		isPkg   bool
		locator string
	}
	var items []item
	for _, p := range a.Packages {
		if p.Deviation > 0 {
			items = append(items, item{label: p.ID, score: p.Score, isPkg: true})
		}
		for _, u := range p.Units {
			if u.Deviation > 0 {
				items = append(items, item{label: u.ID, score: u.Score, locator: fmt.Sprintf("%s:%d", u.File, u.Line)})
			}
		}
	}
	if len(items) == 0 {
		fmt.Fprintln(w, "Nothing deviates from the baseline.")
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score.Deviation > items[j].score.Deviation })
	if len(items) > o.top {
		items = items[:o.top]
	}
	fmt.Fprintln(w, "Most deviant:")
	for _, it := range items {
		kind := "func"
		if it.isPkg {
			kind = "pkg "
		}
		fmt.Fprintf(w, "  %.2f  %s  %s", it.score.Deviation, kind, it.label)
		if it.locator != "" {
			fmt.Fprintf(w, "  (%s)", it.locator)
		}
		fmt.Fprintln(w)
		for _, r := range it.score.Reasons {
			fmt.Fprintf(w, "        %s\n", r)
		}
	}
}

func openBrowser(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", abs)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	return cmd.Start()
}
