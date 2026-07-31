package read

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeNumberedLines(t *testing.T, dir string, name string, n int) string {
	t.Helper()
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		lines[i] = "line-" + strconv.Itoa(i+1)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestReadTextFile_NegativeOffsetTailsFromEnd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeNumberedLines(t, dir, "log.txt", 100)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	tool := &Tool{config: DefaultToolConfig()}
	result, err := tool.readTextFile(context.Background(), path, info, map[string]any{
		"offset": float64(-5),
	})
	if err != nil {
		t.Fatalf("readTextFile: %v", err)
	}

	for _, want := range []string{"line-96", "line-97", "line-98", "line-99", "line-100"} {
		if !strings.Contains(result.Content, want) {
			t.Errorf("expected tail to contain %q, got:\n%s", want, result.Content)
		}
	}
	for _, notWant := range []string{"line-95\n", "line-1\n"} {
		if strings.Contains(result.Content, notWant) {
			t.Errorf("expected tail to NOT contain %q (only the last 5 lines), got:\n%s", notWant, result.Content)
		}
	}
}

func TestReadTextFile_NegativeOffsetBeyondFileLengthReadsFromStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeNumberedLines(t, dir, "short.txt", 10)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	tool := &Tool{config: DefaultToolConfig()}
	result, err := tool.readTextFile(context.Background(), path, info, map[string]any{
		"offset": float64(-500), // file only has 10 lines
	})
	if err != nil {
		t.Fatalf("readTextFile: %v", err)
	}
	if !strings.Contains(result.Content, "line-1") {
		t.Errorf("expected a negative offset beyond file length to clamp to the start, got:\n%s", result.Content)
	}
}

func TestReadTextFile_PositiveOffsetUnaffected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeNumberedLines(t, dir, "log.txt", 20)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	tool := &Tool{config: DefaultToolConfig()}
	result, err := tool.readTextFile(context.Background(), path, info, map[string]any{
		"offset": float64(5),
		"limit":  float64(3),
	})
	if err != nil {
		t.Fatalf("readTextFile: %v", err)
	}
	for _, want := range []string{"line-5", "line-6", "line-7"} {
		if !strings.Contains(result.Content, want) {
			t.Errorf("expected positive-offset behavior unchanged, missing %q, got:\n%s", want, result.Content)
		}
	}
	if strings.Contains(result.Content, "line-8\n") {
		t.Errorf("expected limit=3 to still cap output, got:\n%s", result.Content)
	}
}

func TestValidateInput_NegativeOffsetPassedThroughUnclamped(t *testing.T) {
	t.Parallel()
	tool := &Tool{}
	normalized, err := tool.ValidateInput(context.Background(), map[string]any{
		"file_path": "irrelevant.txt",
		"offset":    float64(-42),
	})
	if err != nil {
		t.Fatalf("ValidateInput: %v", err)
	}
	if got, ok := normalized["offset"].(float64); !ok || got != -42 {
		t.Errorf("expected offset to pass through as -42, got %v", normalized["offset"])
	}
}
