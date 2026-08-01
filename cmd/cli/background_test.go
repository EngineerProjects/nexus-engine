package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KPO-Tech/seshat/pkg/runtimepath"
)

func TestBackgroundSessionResolveByIDPrefixAndName(t *testing.T) {
	t.Setenv(runtimepath.EnvRuntimeRoot, t.TempDir())
	now := time.Now().UTC().Format(time.RFC3339)
	session := backgroundSession{
		ID:        "bg-abcdef123456",
		Name:      "audit",
		PID:       999999,
		Cwd:       t.TempDir(),
		Status:    backgroundStatusFailed,
		StartedAt: now,
		UpdatedAt: now,
		StdoutLog: filepath.Join(t.TempDir(), "out.log"),
		StderrLog: filepath.Join(t.TempDir(), "err.log"),
	}
	if err := saveBackgroundSession(session); err != nil {
		t.Fatalf("save background session: %v", err)
	}

	byPrefix, err := resolveBackgroundSession("bg-abc")
	if err != nil {
		t.Fatalf("resolve by prefix: %v", err)
	}
	if byPrefix.ID != session.ID {
		t.Fatalf("expected %s, got %s", session.ID, byPrefix.ID)
	}
	byName, err := resolveBackgroundSession("audit")
	if err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	if byName.ID != session.ID {
		t.Fatalf("expected %s, got %s", session.ID, byName.ID)
	}
}

func TestBackgroundNameAvailableOnlyBlocksLiveSessions(t *testing.T) {
	t.Setenv(runtimepath.EnvRuntimeRoot, t.TempDir())
	now := time.Now().UTC().Format(time.RFC3339)
	if err := saveBackgroundSession(backgroundSession{
		ID:        "bg-live",
		Name:      "shared",
		PID:       os.Getpid(),
		Status:    backgroundStatusRunning,
		StartedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save live session: %v", err)
	}
	if err := ensureBackgroundNameAvailable("shared"); err == nil {
		t.Fatal("expected live name to be unavailable")
	}
	if _, err := markBackgroundSessionStatus("bg-live", backgroundStatusKilled); err != nil {
		t.Fatalf("mark killed: %v", err)
	}
	if err := ensureBackgroundNameAvailable("shared"); err != nil {
		t.Fatalf("expected terminal name to be reusable: %v", err)
	}
}

func TestRunBackgroundPSPrintsTrackedSessions(t *testing.T) {
	t.Setenv(runtimepath.EnvRuntimeRoot, t.TempDir())
	now := time.Now().UTC().Format(time.RFC3339)
	if err := saveBackgroundSession(backgroundSession{
		ID:        "bg-print",
		Name:      "printable",
		PID:       999999,
		Status:    backgroundStatusFailed,
		Provider:  "openai",
		Model:     "gpt-test",
		StartedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	var out bytes.Buffer
	if err := runBackgroundPS(nil, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("ps: %v", err)
	}
	got := out.String()
	for _, want := range []string{"bg-print", "printable", "failed", "openai:gpt-test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected ps output to contain %q, got:\n%s", want, got)
		}
	}
}
