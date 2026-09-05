// Package dataflow is a deterministic node-graph execution engine: HTTP
// calls, filters, data transforms, database queries, and similar steps that
// don't need an LLM turn to run. It complements pkg/workflow (a multi-agent
// DAG where every node is a Prompt) rather than replacing it — a dataflow
// graph can invoke a pkg/workflow.Definition as a single "subworkflow" node
// (see builtin.go) when a step genuinely needs judgment, without either
// engine reimplementing the other's execution model.
package dataflow

import "time"

// Definition is a node graph: which node types run, their parameters, and
// how each node's output routes to downstream nodes.
type Definition struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Nodes       []Node `json:"nodes" yaml:"nodes"`
	// PinnedData freezes a node's output to a fixed set of items instead of
	// actually running it - Run checks this (keyed by node ID) before
	// invoking a node's own Execute, mirroring n8n's IPinData. Kept
	// separate from each Node's own Parameters, since "frozen test output"
	// is a different concept from real node configuration - a pinned
	// node's ValidateParameters/Execute (and even its registry lookup) are
	// never called at all, so a graph can be tested with a node whose real
	// config is incomplete, or whose type isn't implemented yet.
	PinnedData map[string][]Item `json:"pinned_data,omitempty" yaml:"pinned_data,omitempty"`
}

// Node is one step in the graph. Type selects the NodeExecutor (registered
// by name in a Registry, e.g. "http_request", "if", "agent"). Connections
// maps an output port name to the IDs of nodes that receive items emitted on
// that port — most node types only ever emit on "main"; conditional nodes
// (if/switch) emit on named ports like "true"/"false" instead.
type Node struct {
	ID          string              `json:"id" yaml:"id"`
	Type        string              `json:"type" yaml:"type"`
	Parameters  map[string]any      `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Connections map[string][]string `json:"connections,omitempty" yaml:"connections,omitempty"`
}

// Item is one record flowing through the graph — the dataflow analog of a
// single JSON object. A node's input/output is always a slice of Item, never
// a single bare value, so merge/fan-out behave uniformly across node types.
type Item map[string]any

// Output is what a node execution produces, split by output port. Use
// Main(items) for the common single-port case.
type Output struct {
	Ports map[string][]Item
}

// Main wraps items as a single-port ("main") Output — the shape almost every
// node type other than a conditional (if/switch) needs.
func Main(items []Item) Output {
	return Output{Ports: map[string][]Item{"main": items}}
}

// ItemSource identifies where one received input item came from - which
// upstream node produced it and on which output port. A zero value
// (Node == "") means the item came from this run's own seed input (Run's
// `input` parameter), not from another node.
type ItemSource struct {
	Node string `json:"node,omitempty"`
	Port string `json:"port,omitempty"`
}

// NodeResult is the recorded outcome of one node's execution within a Run.
type NodeResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Success bool   `json:"success"`
	Skipped bool   `json:"skipped,omitempty"`
	// Pinned is true when this result came from Definition.PinnedData
	// rather than a real Execute call - lets a trace/UI consumer tell
	// "this node actually ran" apart from "this used frozen test data".
	Pinned bool `json:"pinned,omitempty"`
	// Input/InputSource are exactly what this node received and, index for
	// index, where each item came from - populated by Run's own scheduling
	// loop (see engine.go), never by the node's own Execute, so every node
	// type gets this for free with no interface change.
	Input       []Item       `json:"input,omitempty"`
	InputSource []ItemSource `json:"input_source,omitempty"`
	// Output is the same single-port view every existing consumer already
	// reads (unchanged) - OutputByPort is the real per-port breakdown Output
	// collapses away (see executeNode).
	Output       []Item            `json:"output,omitempty"`
	OutputByPort map[string][]Item `json:"output_by_port,omitempty"`
	Error        string            `json:"error,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	EndedAt      time.Time         `json:"ended_at"`
	Duration     time.Duration     `json:"duration"`
}

// Result is the outcome of a full graph Run.
type Result struct {
	Name      string                `json:"name"`
	Success   bool                  `json:"success"`
	StartedAt time.Time             `json:"started_at"`
	EndedAt   time.Time             `json:"ended_at"`
	Duration  time.Duration         `json:"duration"`
	Results   map[string]NodeResult `json:"results"`
	// Order is execution order, in the same level-batched order Run used —
	// mirrors pkg/workflow.Result.Order.
	Order []string `json:"order"`
}
