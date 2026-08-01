package repomap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildExtractsGoSymbolsAndFocuses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeFile(t, root, "main.go", `package main

type Server struct{}

func NewServer(name string) *Server { return &Server{} }
func (s *Server) Run(ctx string) error { return nil }
`)
	writeFile(t, root, "internal/quiet/quiet.go", `package quiet

func helper() {}
`)

	m, err := Build(context.Background(), Options{
		Root:         root,
		FocusSymbols: []string{"NewServer"},
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if m.FilesIncluded != 2 {
		t.Fatalf("expected 2 mapped files, got %d", m.FilesIncluded)
	}
	if got := filepath.ToSlash(m.Entries[0].Path); got != "main.go" {
		t.Fatalf("expected focused main.go first, got %q", got)
	}
	rendered := Render(m, 256)
	for _, want := range []string{"struct Server", "func NewServer", "func (*Server) Run"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected rendered map to contain %q:\n%s", want, rendered)
		}
	}
}

func TestRenderTruncatesToBudget(t *testing.T) {
	m := &Map{
		Root:          "/repo",
		FilesScanned:  1,
		FilesIncluded: 1,
		Entries: []Entry{{
			Path:    "large.go",
			Package: "large",
			Symbols: []Symbol{
				{Signature: "func A()"},
				{Signature: strings.Repeat("func VeryLongName()", 100)},
			},
		}},
	}
	rendered := Render(m, 8)
	if !strings.Contains(rendered, "truncated") {
		t.Fatalf("expected truncation marker, got %q", rendered)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
