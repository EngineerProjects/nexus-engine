package dataflow

import (
	"context"

	"github.com/KPO-Tech/seshat/pkg/workflow"
)

// Runtime carries the per-run dependencies node executors need, injected by
// the caller (seshat-backend/seshat-server for multi-tenant secret
// resolution, internal/automation for the agent/subworkflow callers) rather
// than owned by this package — dataflow itself never stores a credential or
// holds an sdk.Client. This is the same "compose through an interface,
// don't couple bounded contexts" split already used for iam/automation in
// seshat-ai's seshat-server.
type Runtime struct {
	Secrets     SecretResolver
	Agent       AgentCaller
	Subworkflow SubworkflowRunner
}

// SecretResolver resolves an opaque reference (e.g. a ConnectorAccount ID,
// or a provider-setting ID) to the secret value a node needs — a database
// password, an API key, an OAuth token. nil is valid: a graph with no node
// that needs a secret never calls it.
type SecretResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// AgentCaller runs a single prompt through a scoped agent turn — what the
// built-in "agent" node type (see builtin.go) delegates to. The concrete
// implementation (e.g. wrapping sdk.Client.Ask) lives with the caller, not
// here, so dataflow never imports pkg/sdk directly.
type AgentCaller interface {
	Ask(ctx context.Context, agentSlug, prompt string) (string, error)
}

// SubworkflowRunner executes a pkg/workflow.Definition — what the built-in
// "subworkflow" node type delegates to, for a step that genuinely needs a
// multi-agent chain (triage -> draft -> verify) rather than a single prompt.
type SubworkflowRunner interface {
	Run(ctx context.Context, def workflow.Definition) (workflow.Result, error)
}
