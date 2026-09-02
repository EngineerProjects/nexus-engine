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

// NodeResult is the recorded outcome of one node's execution within a Run.
type NodeResult struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Success   bool          `json:"success"`
	Skipped   bool          `json:"skipped,omitempty"`
	Output    []Item        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
	Duration  time.Duration `json:"duration"`
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
