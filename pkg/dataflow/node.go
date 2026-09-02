package dataflow

import (
	"context"
	"fmt"
	"sync"
)

// NodeExecutor is what every node type implements — a thin adapter between
// the graph engine and one deterministic operation (an HTTP call, a filter,
// a database query, ...). Implementations should not hold per-run state;
// Runtime carries whatever per-run dependencies (secrets, callers) a node
// needs.
type NodeExecutor interface {
	Execute(ctx context.Context, rt *Runtime, input []Item, params map[string]any) (Output, error)
	Description() NodeDescription
	ValidateParameters(params map[string]any) error
}

// NodeDescription is metadata about a node type, for catalogs/authoring UIs
// (an agent building a Definition can be handed a list of these).
type NodeDescription struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Properties  []NodeProperty `json:"properties,omitempty"`
}

// NodePropertyType is the set of parameter field widgets an authoring UI
// needs to render a node's Properties as a real form instead of raw JSON.
// Deliberately a small, flat subset — not a port of n8n's ~20-type
// INodeProperties system, which exists to serve thousands of third-party
// nodes. A node whose parameters don't fit this shape (a nested
// sub-workflow definition, a free-form key/value map with user-defined
// keys) should leave Properties empty rather than force a bad fit; the
// authoring UI falls back to raw JSON for those, which is correct, not a
// shortfall.
type NodePropertyType string

const (
	PropString    NodePropertyType = "string"
	PropText      NodePropertyType = "text" // multiline string
	PropNumber    NodePropertyType = "number"
	PropBoolean   NodePropertyType = "boolean"
	PropOptions   NodePropertyType = "options"   // single-select dropdown, static Options list
	PropJSON      NodePropertyType = "json"      // free-form object/array, still authored as JSON text
	PropSecretRef NodePropertyType = "secretRef" // a named secret's value is resolved by the caller, never authored inline - see dataflow.SecretResolver
)

type NodePropertyOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// DisplayCondition gates a property's visibility on another property's
// current value — the simplest useful subset of n8n's displayOptions.show
// (no hide, no operators beyond equality-in-a-set), sufficient for every
// current node's conditional fields (e.g. a database node's operation
// picker gating which of query/args apply).
type DisplayCondition struct {
	Field  string   `json:"field"`
	Equals []string `json:"equals"`
}

// NodeProperty describes one parameter field for an authoring UI's form
// renderer. Options a node can only resolve at runtime (a tenant's
// connector accounts, an agent's configured personas) have no place to
// live here — Description() is a stateless method — so such a field stays
// a plain string property; the UI can't render a dynamic dropdown for it.
type NodeProperty struct {
	Name        string               `json:"name"`
	DisplayName string               `json:"displayName"`
	Type        NodePropertyType     `json:"type"`
	Required    bool                 `json:"required,omitempty"`
	Default     any                  `json:"default,omitempty"`
	Placeholder string               `json:"placeholder,omitempty"`
	Description string               `json:"description,omitempty"`
	Options     []NodePropertyOption `json:"options,omitempty"`
	DisplayIf   *DisplayCondition    `json:"displayIf,omitempty"`
}

// Base provides the boilerplate most NodeExecutor implementations share —
// embed it and override Execute (and ValidateParameters when a node has
// required parameters).
type Base struct {
	description NodeDescription
}

func NewBase(description NodeDescription) Base {
	return Base{description: description}
}

func (b Base) Description() NodeDescription { return b.description }

// ValidateParameters is a no-op default; nodes with required parameters
// should override it.
func (b Base) ValidateParameters(map[string]any) error { return nil }

func StringParam(params map[string]any, name, fallback string) string {
	if v, ok := params[name].(string); ok {
		return v
	}
	return fallback
}

func IntParam(params map[string]any, name string, fallback int) int {
	switch v := params[name].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return fallback
	}
}

func BoolParam(params map[string]any, name string, fallback bool) bool {
	if v, ok := params[name].(bool); ok {
		return v
	}
	return fallback
}

// Registry maps node type names to their executors. A Run needs a Registry
// covering every Type referenced by the Definition being executed.
type Registry struct {
	mu        sync.RWMutex
	executors map[string]NodeExecutor
}

func NewRegistry() *Registry {
	return &Registry{executors: make(map[string]NodeExecutor)}
}

func (r *Registry) Register(nodeType string, executor NodeExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[nodeType] = executor
}

func (r *Registry) Get(nodeType string) (NodeExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	executor, ok := r.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("dataflow: no executor registered for node type %q", nodeType)
	}
	return executor, nil
}

// Types lists every registered node type's description, for authoring
// surfaces (e.g. a tool the Inbox Agent uses to see what's available).
func (r *Registry) Types() []NodeDescription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]NodeDescription, 0, len(r.executors))
	for _, executor := range r.executors {
		result = append(result, executor.Description())
	}
	return result
}
