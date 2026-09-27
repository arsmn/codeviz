package goparser

import (
	"go/ast"
	"go/token"
	"sort"

	"github.com/arsmn/codeviz/internal/domain"
)

// extractUnits turns every function and method with a body into a Unit.
func extractUnits(fset *token.FileSet, pf *pkgFiles) []domain.Unit {
	top, methods := packageSymbols(pf.files)

	var units []domain.Unit
	for i, f := range pf.files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			recv, ptr := receiverType(fd)
			name, qualified := fd.Name.Name, fd.Name.Name
			if recv != "" {
				name = recv + "." + fd.Name.Name
				qualified = recv + "." + fd.Name.Name
				if ptr {
					qualified = "(*" + recv + ")." + fd.Name.Name
				}
			}
			start, end := fset.Position(fd.Pos()), fset.Position(fd.End())
			units = append(units, domain.Unit{
				ID:         pf.id + "." + qualified,
				Name:       name,
				File:       pf.paths[i],
				Line:       start.Line,
				LOC:        end.Line - start.Line + 1,
				Cyclomatic: cyclomatic(fd),
				Refs:       references(fd, name, recv, top, methods),
			})
		}
	}
	return units
}

// packageSymbols collects top-level names, and method names mapped to their
// "Type.Method" symbols (used to resolve x.Method() without type checking).
func packageSymbols(files []*ast.File) (top map[string]bool, methods map[string][]string) {
	top, methods = map[string]bool{}, map[string][]string{}
	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if recv, _ := receiverType(d); recv != "" {
					methods[d.Name.Name] = append(methods[d.Name.Name], recv+"."+d.Name.Name)
				} else if d.Name.Name != "init" {
					top[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						top[s.Name.Name] = true
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.Name != "_" {
								top[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return top, methods
}

// references lists the package-local symbols a function touches. It is a
// syntactic approximation: shadowed names and same-named methods on
// different types can over-connect, which only errs towards higher cohesion.
func references(fd *ast.FuncDecl, self, recv string, top map[string]bool, methods map[string][]string) []string {
	set := map[string]bool{}
	if recv != "" {
		set[recv] = true
	}
	skip := map[*ast.Ident]bool{fd.Name: true}
	ast.Inspect(fd, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			skip[n.Sel] = true
			for _, m := range methods[n.Sel.Name] {
				set[m] = true
			}
		case *ast.Ident:
			if !skip[n] && top[n.Name] {
				set[n.Name] = true
			}
		}
		return true
	})
	delete(set, self)
	refs := make([]string, 0, len(set))
	for s := range set {
		refs = append(refs, s)
	}
	sort.Strings(refs)
	return refs
}

// cyclomatic follows gocyclo: 1 + one per if, for, range, non-default case,
// non-default select case, && and ||. Closures count towards their parent.
func cyclomatic(fd *ast.FuncDecl) int {
	c := 1
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			c++
		case *ast.CaseClause:
			if n.List != nil {
				c++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				c++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}

func receiverType(fd *ast.FuncDecl) (name string, pointer bool) {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return "", false
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		pointer, t = true, s.X
	}
	switch g := t.(type) { // generic receivers: T[K] or T[K, V]
	case *ast.IndexExpr:
		t = g.X
	case *ast.IndexListExpr:
		t = g.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name, pointer
	}
	return "?", pointer
}
