package sdk

import (
	"strings"
	"testing"
)

func TestDraftWorkflow(t *testing.T) {
	result, err := DraftWorkflow(WorkflowDraftOptions{
		Format: "json",
		Definition: WorkflowDefinition{
			Name:  "sdk-draft",
			Nodes: []WorkflowNode{{ID: "one", Prompt: "Do one thing."}},
		},
	})
	if err != nil {
		t.Fatalf("draft workflow: %v", err)
	}
	if result.Format != "json" {
		t.Fatalf("expected json format, got %q", result.Format)
	}
	if !strings.Contains(result.Content, `"name": "sdk-draft"`) {
		t.Fatalf("missing rendered workflow:\n%s", result.Content)
	}
}
