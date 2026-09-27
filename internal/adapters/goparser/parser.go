// Package goparser is the Go SourceParser adapter, built on go/parser and
// go/ast only (no type checking), so it works on code that doesn't build.
package goparser

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/arsmn/codeviz/internal/domain"
)

type Parser struct {
	IncludeTests     bool // include _test.go files
	IncludeGenerated bool // include files marked "Code generated ... DO NOT EDIT."
}

type pkgFiles struct {
	id, importPath string
	files          []*ast.File
	paths          []string // module-relative, parallel to files
	loc            int
	imports        map[string]bool
}

func (p Parser) Parse(root string) (domain.Codebase, []string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return domain.Codebase{}, nil, err
	}
	if info, err := os.Stat(root); err != nil {
		return domain.Codebase{}, nil, err
	} else if !info.IsDir() {
		return domain.Codebase{}, nil, fmt.Errorf("%s is not a directory", root)
	}

	var warnings []string
	modRoot, modPath := findModule(root)
	if modRoot == "" {
		modRoot = root
		warnings = append(warnings, "no go.mod found; imports between packages cannot be resolved")
	}

	dirs, err := p.goFilesByDir(root)
	if err != nil {
		return domain.Codebase{}, warnings, err
	}

	fset := token.NewFileSet()
	var pkgs []*pkgFiles
	for _, dir := range sortedKeys(dirs) {
		pf, warns := p.parseDir(fset, modRoot, modPath, dir, dirs[dir])
		warnings = append(warnings, warns...)
		if pf != nil {
			pkgs = append(pkgs, pf)
		}
	}

	byImportPath := make(map[string]string, len(pkgs))
	for _, pf := range pkgs {
		byImportPath[pf.importPath] = pf.id
	}

	cb := domain.Codebase{Name: importPathOf(modRoot, modPath, root), Language: "go"}
	for _, pf := range pkgs {
		pkg := domain.Package{ID: pf.id, Files: len(pf.files), LOC: pf.loc}
		for imp := range pf.imports {
			if id, ok := byImportPath[imp]; ok {
				pkg.Imports = append(pkg.Imports, id)
			} else {
				pkg.ExternalImports++
			}
		}
		sort.Strings(pkg.Imports)
		pkg.Units = extractUnits(fset, pf)
		cb.Packages = append(cb.Packages, pkg)
	}
	return cb, warnings, nil
}

// goFilesByDir walks root, skipping hidden, vendor and testdata directories
// and nested modules.
func (p Parser) goFilesByDir(root string) (map[string][]string, error) {
	dirs := map[string][]string{}
	err := filepath.WalkDir(root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if pth == root {
				return nil
			}
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "vendor" || name == "testdata" || name == "node_modules" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(pth, "go.mod")); err == nil {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".go") && (p.IncludeTests || !strings.HasSuffix(name, "_test.go")) {
			dir := filepath.Dir(pth)
			dirs[dir] = append(dirs[dir], name)
		}
		return nil
	})
	return dirs, err
}

func (p Parser) parseDir(fset *token.FileSet, modRoot, modPath, dir string, names []string) (*pkgFiles, []string) {
	var warnings []string
	type parsed struct {
		file *ast.File
		path string
	}
	byPkg := map[string][]parsed{}
	for _, name := range names {
		full := filepath.Join(dir, name)
		rel := relSlash(modRoot, full)
		if ok, err := build.Default.MatchFile(dir, name); err != nil || !ok {
			continue // excluded by build constraints on this platform
		}
		f, err := parser.ParseFile(fset, full, nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipped %s: %v", rel, firstLine(err)))
			continue
		}
		if !p.IncludeGenerated && ast.IsGenerated(f) {
			continue
		}
		// External test packages (foo_test) are folded into foo.
		pkgName := strings.TrimSuffix(f.Name.Name, "_test")
		byPkg[pkgName] = append(byPkg[pkgName], parsed{f, rel})
	}
	if len(byPkg) == 0 {
		return nil, warnings
	}
	// A directory should hold one package; keep the one with the most files.
	best := ""
	for name, fs := range byPkg {
		if best == "" || len(fs) > len(byPkg[best]) || (len(fs) == len(byPkg[best]) && name < best) {
			best = name
		}
	}
	for name := range byPkg {
		if name != best {
			warnings = append(warnings, fmt.Sprintf("%s: ignored files of package %q (kept %q)", relSlash(modRoot, dir), name, best))
		}
	}

	importPath := importPathOf(modRoot, modPath, dir)
	id := relSlash(modRoot, dir)
	if id == "." {
		id = path.Base(importPath)
		if id == "." || id == "/" {
			id = filepath.Base(dir)
		}
	}
	pf := &pkgFiles{id: id, importPath: importPath, imports: map[string]bool{}}
	for _, ps := range byPkg[best] {
		pf.files = append(pf.files, ps.file)
		pf.paths = append(pf.paths, ps.path)
		pf.loc += fset.File(ps.file.Pos()).LineCount()
		for _, imp := range ps.file.Imports {
			if s, err := strconv.Unquote(imp.Path.Value); err == nil && s != "C" {
				pf.imports[s] = true
			}
		}
	}
	delete(pf.imports, importPath) // external test package importing its subject
	return pf, warnings
}

// importPathOf maps a directory to its Go import path. The standard library's
// module is named "std" but its import paths carry no prefix.
func importPathOf(modRoot, modPath, dir string) string {
	rel := relSlash(modRoot, dir)
	switch {
	case modPath == "" || modPath == "std":
		return rel
	case rel == ".":
		return modPath
	default:
		return modPath + "/" + rel
	}
}

// findModule walks up from dir to the nearest go.mod.
func findModule(dir string) (root, modPath string) {
	for d := dir; ; d = filepath.Dir(d) {
		if mp, err := readModulePath(filepath.Join(d, "go.mod")); err == nil {
			return d, mp
		}
		if filepath.Dir(d) == d {
			return "", ""
		}
	}
}

func readModulePath(gomod string) (string, error) {
	f, err := os.Open(gomod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if rest, ok := strings.CutPrefix(line, "module"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
			mp := strings.TrimSpace(rest)
			if uq, err := strconv.Unquote(mp); err == nil {
				mp = uq
			}
			return mp, nil
		}
	}
	return "", errors.New("no module directive")
}

func relSlash(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
