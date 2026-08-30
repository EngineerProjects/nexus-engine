package sdk

import "testing"

// TestReloadPreToolHooksReplacesRatherThanAccumulates is the pkg/sdk-level
// contract test for the live-reload primitive
// docs/helps/audit-2026-08-29-openwork-den-comparison.md § 7 needed: unlike
// ClientConfig.PreToolHooks (only ever read once, inside NewClient),
// ReloadPreToolHooks can be called repeatedly with a fresh set and must
// never leave a stale prior registration running alongside the new one -
// the exact behavior TestOrchestratorRemoveHookStopsItFromFiring proves at
// the orchestrator layer; this just confirms the wiring from Client down to
// that layer holds together with no error across repeated calls, including
// clearing every hook by reloading with an empty list.
func TestReloadPreToolHooksReplacesRatherThanAccumulates(t *testing.T) {
	client, err := NewClient(&ClientConfig{
		PersistSessions: false,
		PreToolHooks: []PreToolHookConfig{
			{Command: "exit 0"},
		},
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := client.ReloadPreToolHooks([]PreToolHookConfig{
		{Matcher: "bash", Command: "exit 0"},
	}); err != nil {
		t.Fatalf("first ReloadPreToolHooks failed: %v", err)
	}

	if err := client.ReloadPreToolHooks([]PreToolHookConfig{
		{Matcher: "bash", Command: "exit 0"},
		{Matcher: "shell", Command: "exit 0"},
	}); err != nil {
		t.Fatalf("second ReloadPreToolHooks failed: %v", err)
	}

	if err := client.ReloadPreToolHooks(nil); err != nil {
		t.Fatalf("ReloadPreToolHooks with an empty set failed: %v", err)
	}
}

func TestReloadPreToolHooksOnNilClientReturnsError(t *testing.T) {
	var client *Client
	if err := client.ReloadPreToolHooks(nil); err == nil {
		t.Fatal("expected an error reloading hooks on a nil client")
	}
}
