package workflow

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunExecutesDependenciesBeforeChildren(t *testing.T) {
	def := Definition{Name: "test", Nodes: []Node{
		{ID: "a", Prompt: "first"},
		{ID: "b", Prompt: "second", Needs: []string{"a"}},
	}}
	var seen []string
	result, err := Run(context.Background(), def, ExecutorFunc(func(_ context.Context, node Node, inputs map[string]NodeResult) (string, error) {
		seen = append(seen, node.ID)
		if node.ID == "b" && inputs["a"].Output != "out-a" {
			t.Fatalf("expected b to receive a output, got %#v", inputs)
		}
		return "out-" + node.ID, nil
	}), Options{})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if !result.Success {
		t.Fatal("expected workflow success")
	}
	if strings.Join(seen, ",") != "a,b" {
		t.Fatalf("unexpected execution order: %v", seen)
	}
}

func TestRunExecutesReadyNodesConcurrently(t *testing.T) {
	def := Definition{Name: "parallel", Nodes: []Node{
		{ID: "a", Prompt: "a"},
		{ID: "b", Prompt: "b"},
	}}
	var mu sync.Mutex
	running := 0
	maxRunning := 0
	_, err := Run(context.Background(), def, ExecutorFunc(func(_ context.Context, node Node, inputs map[string]NodeResult) (string, error) {
		mu.Lock()
		running++
		if running > maxRunning {
			maxRunning = running
		}
		mu.Unlock()
		time.Sleep(25 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
		return node.ID, nil
	}), Options{MaxParallel: 2})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if maxRunning < 2 {
		t.Fatalf("expected concurrent execution, maxRunning=%d", maxRunning)
	}
}

func TestValidateRejectsCycles(t *testing.T) {
	err := Validate(Definition{Nodes: []Node{
		{ID: "a", Prompt: "a", Needs: []string{"b"}},
		{ID: "b", Prompt: "b", Needs: []string{"a"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestBuildNodePromptIncludesDependencyOutputs(t *testing.T) {
	prompt := BuildNodePrompt(Node{ID: "final", Prompt: "summarize"}, map[string]NodeResult{
		"research": {ID: "research", Success: true, Output: "facts"},
	})
	if !strings.Contains(prompt, "[research]") || !strings.Contains(prompt, "facts") || !strings.Contains(prompt, "summarize") {
		t.Fatalf("prompt missing expected content:\n%s", prompt)
	}
}

func TestBuildNodePromptIncludesRoleGuidance(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want string
	}{
		{
			name: "verifier",
			node: Node{ID: "verify", Kind: "verifier", Prompt: "Check the answer."},
			want: "Role: verifier.",
		},
		{
			name: "critic",
			node: Node{ID: "critique", Kind: "critic", Prompt: "Review the plan."},
			want: "Role: critic.",
		},
		{
			name: "router",
			node: Node{ID: "route", Kind: "router", Prompt: "Pick a path.", Routes: []string{"code", "research"}},
			want: "Allowed routes: code, research",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := BuildNodePrompt(tt.node, nil)
			if !strings.Contains(prompt, tt.want) {
				t.Fatalf("prompt missing %q:\n%s", tt.want, prompt)
			}
		})
	}
}

func TestValidateRejectsUnknownNodeKind(t *testing.T) {
	err := Validate(Definition{Nodes: []Node{{ID: "x", Kind: "mystery", Prompt: "Do it."}}})
	if err == nil || !strings.Contains(err.Error(), "unsupported kind") {
		t.Fatalf("expected unsupported kind error, got %v", err)
	}
}

func TestValidateRejectsRouterWithoutRoutes(t *testing.T) {
	err := Validate(Definition{Nodes: []Node{{ID: "route", Kind: "router", Prompt: "Pick."}}})
	if err == nil || !strings.Contains(err.Error(), "must declare at least one route") {
		t.Fatalf("expected routes error, got %v", err)
	}
}

func TestRunMarksDependentNodeFailedWhenDependencyFails(t *testing.T) {
	def := Definition{Nodes: []Node{
		{ID: "a", Prompt: "a"},
		{ID: "b", Prompt: "b", Needs: []string{"a"}},
	}}
	result, err := Run(context.Background(), def, ExecutorFunc(func(_ context.Context, node Node, _ map[string]NodeResult) (string, error) {
		if node.ID == "a" {
			return "", errors.New("boom")
		}
		return "should not run", nil
	}), Options{})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if result.Success {
		t.Fatal("expected workflow failure")
	}
	if result.Results["b"].Error != `dependency "a" failed` {
		t.Fatalf("unexpected dependent error: %#v", result.Results["b"])
	}
}
