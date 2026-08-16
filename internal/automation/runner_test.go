package automation

import (
	"testing"

	"github.com/KPO-Tech/seshat/internal/providers"
	"github.com/KPO-Tech/seshat/pkg/sdk"
)

// TestBuildClientConfigPropagatesRequireSandbox is a network-free plumbing
// check (see cloud_execution_test.go's own "no LLM network call in a unit
// test" policy in seshat-ai/seshat-server, mirrored here): it doesn't run
// Execute end to end, just proves RunnerConfig.RequireSandbox actually
// reaches the sdk.ClientConfig a real execution would use.
func TestBuildClientConfigPropagatesRequireSandbox(t *testing.T) {
	runner, err := NewRunner(RunnerConfig{
		ProviderConfig: &providers.Config{APIKey: "test-key"},
		MaxTokens:      1024,
		RequireSandbox: true,
	})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	cfg := runner.buildClientConfig(sdk.ModelIdentifier{Provider: "anthropic", Model: "claude-sonnet-5"})
	if !cfg.RequireSandbox {
		t.Fatal("expected RunnerConfig.RequireSandbox=true to propagate to sdk.ClientConfig.RequireSandbox")
	}
}

func TestBuildClientConfigDefaultsRequireSandboxToFalse(t *testing.T) {
	runner, err := NewRunner(RunnerConfig{
		ProviderConfig: &providers.Config{APIKey: "test-key"},
		MaxTokens:      1024,
	})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	cfg := runner.buildClientConfig(sdk.ModelIdentifier{Provider: "anthropic", Model: "claude-sonnet-5"})
	if cfg.RequireSandbox {
		t.Fatal("expected RequireSandbox to default to false for a single-user embedding, matching today's behavior")
	}
}
