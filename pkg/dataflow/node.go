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
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
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
