package read

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSourceAndSidecar(t *testing.T, dir, base, ext, sourceContent, mdContent string, sourceAge, mdAge time.Duration) (string, os.FileInfo) {
	t.Helper()
	sourcePath := filepath.Join(dir, base+ext)
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	now := time.Now()
	if err := os.Chtimes(sourcePath, now.Add(-sourceAge), now.Add(-sourceAge)); err != nil {
		t.Fatalf("chtimes source: %v", err)
	}
	if mdContent != "" {
		mdPath := filepath.Join(dir, base+".md")
		if err := os.WriteFile(mdPath, []byte(mdContent), 0o600); err != nil {
			t.Fatalf("write sidecar: %v", err)
		}
		if err := os.Chtimes(mdPath, now.Add(-mdAge), now.Add(-mdAge)); err != nil {
			t.Fatalf("chtimes sidecar: %v", err)
		}
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	return sourcePath, info
}

func TestSidecarMarkdownUsesFreshCache(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sourcePath, info := writeSourceAndSidecar(t, dir, "deck", ".pptx", "binary-ish", "# Deck\n\nSlide notes", time.Hour, time.Minute)

	got := sidecarMarkdown(sourcePath, info)
	if got != "# Deck\n\nSlide notes" {
		t.Fatalf("expected cached markdown, got %q", got)
	}
}

func TestSidecarMarkdownMissingReturnsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sourcePath, info := writeSourceAndSidecar(t, dir, "deck", ".pptx", "binary-ish", "", 0, 0)

	if got := sidecarMarkdown(sourcePath, info); got != "" {
		t.Fatalf("expected no cache without a sidecar, got %q", got)
	}
}

func TestSidecarMarkdownStaleOlderThanSourceIsRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Sidecar written BEFORE the source's current mtime - source was replaced/edited since.
	sourcePath, info := writeSourceAndSidecar(t, dir, "deck", ".pptx", "binary-ish", "# Stale", 0, time.Hour)

	if got := sidecarMarkdown(sourcePath, info); got != "" {
		t.Fatalf("expected stale sidecar (older than source) to be rejected, got %q", got)
	}
}

func TestSidecarMarkdownTooOldIsRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sourcePath, info := writeSourceAndSidecar(t, dir, "deck", ".pptx", "binary-ish", "# Ancient", 48*time.Hour, 30*time.Hour)

	if got := sidecarMarkdown(sourcePath, info); got != "" {
		t.Fatalf("expected sidecar older than markdownSidecarMaxAge to be rejected, got %q", got)
	}
}
