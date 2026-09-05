package dataflow

import (
	"context"
	"sync"

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
	// Expr enables expression resolution (ResolveParam/ResolveValue) for any
	// node that opts in - nil (the default for a caller that doesn't set it,
	// and for every existing Runtime{} literal/nil Runtime already in use)
	// means expressions are off, not an error; nodes calling ResolveParam
	// against a nil-Expr Runtime just get their literal values back
	// unresolved.
	Expr ExpressionEvaluator

	nodeOutputsMu sync.RWMutex
	nodeOutputs   map[string][]Item
}

// NodeOutput returns the named node's Output ("main"-collapsed, same shape
// as NodeResult.Output) once Run has recorded it - only for a node that has
// actually finished executing, mirroring n8n's own $('NodeName') execution-
// order requirement. ok is false before that (including for a name that
// will never exist). Safe to call from concurrently-running node
// executions - Run itself is the only writer, under its own lock.
func (rt *Runtime) NodeOutput(name string) ([]Item, bool) {
	if rt == nil {
		return nil, false
	}
	rt.nodeOutputsMu.RLock()
	defer rt.nodeOutputsMu.RUnlock()
	items, ok := rt.nodeOutputs[name]
	return items, ok
}

// recordNodeOutput is called by Run as each node finishes - see engine.go.
func (rt *Runtime) recordNodeOutput(name string, items []Item) {
	if rt == nil {
		return
	}
	rt.nodeOutputsMu.Lock()
	defer rt.nodeOutputsMu.Unlock()
	if rt.nodeOutputs == nil {
		rt.nodeOutputs = make(map[string][]Item)
	}
	rt.nodeOutputs[name] = items
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
