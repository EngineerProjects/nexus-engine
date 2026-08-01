package workflow

import (
	"strings"
	"testing"
)

func TestDraftYAMLNormalizesAndRenders(t *testing.T) {
	result, err := Draft(DraftOptions{
		Format: "yml",
		Definition: Definition{
			Name: " research ",
			Nodes: []Node{
				{ID: " collect ", Prompt: " Find facts. ", MaxTurns: -1},
				{ID: " final ", Prompt: " Conclude. ", Needs: []string{" collect ", ""}},
			},
		},
	})
	if err != nil {
		t.Fatalf("draft workflow: %v", err)
	}
	if result.Format != DraftFormatYAML {
		t.Fatalf("expected yaml format, got %q", result.Format)
	}
	if result.Definition.Name != "research" {
		t.Fatalf("expected normalized name, got %q", result.Definition.Name)
	}
	if result.Definition.Nodes[0].Kind != "" {
		t.Fatalf("expected empty default kind to remain empty, got %q", result.Definition.Nodes[0].Kind)
	}
	if result.Definition.Nodes[0].MaxTurns != 0 {
		t.Fatalf("expected negative max turns to be cleared")
	}
	if got := result.Definition.Nodes[1].Needs; len(got) != 1 || got[0] != "collect" {
		t.Fatalf("expected normalized needs, got %#v", got)
	}
	if !strings.Contains(result.Content, "name: research") || !strings.Contains(result.Content, "- collect") {
		t.Fatalf("rendered yaml missing expected content:\n%s", result.Content)
	}
}

func TestDraftJSON(t *testing.T) {
	result, err := Draft(DraftOptions{
		Format: DraftFormatJSON,
		Definition: Definition{
			Name:  "simple",
			Nodes: []Node{{ID: "one", Prompt: "Do one thing."}},
		},
	})
	if err != nil {
		t.Fatalf("draft workflow: %v", err)
	}
	if result.Format != DraftFormatJSON {
		t.Fatalf("expected json format, got %q", result.Format)
	}
	if !strings.Contains(result.Content, `"name": "simple"`) {
		t.Fatalf("rendered json missing name:\n%s", result.Content)
	}
}

func TestDraftNormalizesRouterRoutes(t *testing.T) {
	result, err := Draft(DraftOptions{
		Definition: Definition{
			Name: "route",
			Nodes: []Node{
				{ID: " code ", Prompt: "Implement."},
				{ID: " research ", Prompt: "Research."},
				{ID: " router ", Kind: " Router ", Prompt: "Pick path.", Routes: []string{" code ", "", "research "}},
			},
		},
	})
	if err != nil {
		t.Fatalf("draft workflow: %v", err)
	}
	router := result.Definition.Nodes[2]
	if router.Kind != "router" {
		t.Fatalf("expected normalized kind, got %q", router.Kind)
	}
	if len(router.Routes) != 2 || router.Routes[0] != "code" || router.Routes[1] != "research" {
		t.Fatalf("expected normalized routes, got %#v", router.Routes)
	}
}

func TestDraftReturnsDiagnosticsForInvalidDefinition(t *testing.T) {
	result, err := Draft(DraftOptions{
		Definition: Definition{
			Name:  "broken",
			Nodes: []Node{{ID: "one"}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", result.Diagnostics)
	}
	if !strings.Contains(result.Diagnostics[0], "prompt is required") {
		t.Fatalf("unexpected diagnostic: %#v", result.Diagnostics)
	}
}
