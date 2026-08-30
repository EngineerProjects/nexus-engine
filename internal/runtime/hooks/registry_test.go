package hooks

import (
	"context"
	"testing"
)

func noopExecute(ctx context.Context, input ToolHookInput) ToolHookResult { return ToolHookResult{} }

func TestRegistryAddOrdersByPriorityDescending(t *testing.T) {
	r := NewRegistry()
	r.Add(ToolHook{ID: "low", Stage: ToolHookStagePre, Priority: 1, Execute: noopExecute})
	r.Add(ToolHook{ID: "high", Stage: ToolHookStagePre, Priority: 10, Execute: noopExecute})
	r.Add(ToolHook{ID: "mid", Stage: ToolHookStagePre, Priority: 5, Execute: noopExecute})

	pre := r.Pre()
	if len(pre) != 3 {
		t.Fatalf("expected 3 pre hooks, got %d", len(pre))
	}
	if pre[0].ID != "high" || pre[1].ID != "mid" || pre[2].ID != "low" {
		t.Fatalf("expected hooks ordered by descending priority, got %v", []string{pre[0].ID, pre[1].ID, pre[2].ID})
	}
}

func TestRegistryByStageFiltersCorrectly(t *testing.T) {
	r := NewRegistry()
	r.Add(ToolHook{ID: "pre-1", Stage: ToolHookStagePre, Execute: noopExecute})
	r.Add(ToolHook{ID: "post-1", Stage: ToolHookStagePost, Execute: noopExecute})

	if pre := r.Pre(); len(pre) != 1 || pre[0].ID != "pre-1" {
		t.Fatalf("expected only pre-1 in Pre(), got %+v", pre)
	}
	if post := r.Post(); len(post) != 1 || post[0].ID != "post-1" {
		t.Fatalf("expected only post-1 in Post(), got %+v", post)
	}
}

// TestRegistryRemoveDropsMatchingID covers the behavior ReloadPreToolHooks
// (pkg/sdk) depends on: replacing a previously-registered hook set rather
// than accumulating duplicate registrations across reloads.
func TestRegistryRemoveDropsMatchingID(t *testing.T) {
	r := NewRegistry()
	r.Add(ToolHook{ID: "shell-pre-tool-hooks", Stage: ToolHookStagePre, Execute: noopExecute})
	r.Add(ToolHook{ID: "unrelated", Stage: ToolHookStagePre, Execute: noopExecute})

	removed := r.Remove("shell-pre-tool-hooks")
	if removed != 1 {
		t.Fatalf("expected 1 hook removed, got %d", removed)
	}
	pre := r.Pre()
	if len(pre) != 1 || pre[0].ID != "unrelated" {
		t.Fatalf("expected only the unrelated hook to remain, got %+v", pre)
	}
}

func TestRegistryRemoveDropsAllMatchingIDs(t *testing.T) {
	r := NewRegistry()
	r.Add(ToolHook{ID: "dup", Stage: ToolHookStagePre, Execute: noopExecute})
	r.Add(ToolHook{ID: "dup", Stage: ToolHookStagePre, Execute: noopExecute})
	r.Add(ToolHook{ID: "keep", Stage: ToolHookStagePre, Execute: noopExecute})

	removed := r.Remove("dup")
	if removed != 2 {
		t.Fatalf("expected both duplicate-ID hooks removed, got %d", removed)
	}
	if pre := r.Pre(); len(pre) != 1 || pre[0].ID != "keep" {
		t.Fatalf("expected only keep to remain, got %+v", pre)
	}
}

func TestRegistryRemoveNonExistentIDIsANoop(t *testing.T) {
	r := NewRegistry()
	r.Add(ToolHook{ID: "keep", Stage: ToolHookStagePre, Execute: noopExecute})

	if removed := r.Remove("does-not-exist"); removed != 0 {
		t.Fatalf("expected 0 removed for a non-existent ID, got %d", removed)
	}
	if pre := r.Pre(); len(pre) != 1 {
		t.Fatalf("expected the existing hook to be untouched, got %+v", pre)
	}
}

func TestRegistryRemoveOnNilRegistryIsSafe(t *testing.T) {
	var r *Registry
	if removed := r.Remove("anything"); removed != 0 {
		t.Fatalf("expected 0 from a nil registry, got %d", removed)
	}
}
