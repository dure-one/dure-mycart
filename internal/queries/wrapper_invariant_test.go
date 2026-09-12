package queries

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestQueriesStayInsideTheDialectWrapper enforces the invariant the whole
// portability design rests on: nothing in this package may reach the
// underlying *sql.DB.
//
// Conn deliberately does not embed *sql.DB, so `?` placeholders and SQLite-only
// fragments cannot escape into a query — but Raw() hands the unbound handle
// back, and it is only a keystroke away. On SQLite a leak compiles and passes
// every test; on PostgreSQL it fails at runtime, on the installation of
// whoever upgraded. Hence a static check.
//
// `q.DB` is not a violation: it *is* the wrapper. Only Raw() — and opening a
// second connection by hand — is.
func TestQueriesStayInsideTheDialectWrapper(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	scanned := 0
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			scanned++
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				position := fset.Position(call.Pos())
				switch {
				case sel.Sel.Name == "Raw":
					t.Errorf("%s: %s calls Raw(); queries must go through the dialect-aware wrapper",
						position, filepath.Base(name))
				case sel.Sel.Name == "Open", sel.Sel.Name == "OpenDB":
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "sql" {
						t.Errorf("%s: %s opens its own database connection; it must use the shared handle",
							position, filepath.Base(name))
					}
				}
				return true
			})
		}
	}

	// A scan that found nothing would pass silently.
	if scanned < 5 {
		t.Fatalf("only %d files scanned; the check is not looking at the package", scanned)
	}
}
