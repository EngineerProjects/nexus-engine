package dataflow

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// funcExecutor lets tests define a node's behavior as a plain function,
// mirroring pkg/workflow's ExecutorFunc.
type funcExecutor struct {
	desc     NodeDescription
	validate func(map[string]any) error
	execute  func(ctx context.Context, rt *Runtime, input []Item, params map[string]any) (Output, error)
}

func (f funcExecutor) Description() NodeDescription { return f.desc }

func (f funcExecutor) ValidateParameters(params map[string]any) error {
	if f.validate == nil {
		return nil
	}
	return f.validate(params)
}

func (f funcExecutor) Execute(ctx context.Context, rt *Runtime, input []Item, params map[string]any) (Output, error) {
	return f.execute(ctx, rt, input, params)
}

func passthrough(nodeType string) funcExecutor {
	return funcExecutor{
		desc: NodeDescription{Type: nodeType},
		execute: func(_ context.Context, _ *Runtime, input []Item, _ map[string]any) (Output, error) {
			return Main(input), nil
		},
	}
}

func TestRunExecutesDependenciesBeforeChildren(t *testing.T) {
	reg := NewRegistry()
	var seen []string
	var mu sync.Mutex
	reg.Register("a", funcExecutor{desc: NodeDescription{Type: "a"}, execute: func(_ context.Context, _ *Runtime, input []Item, _ map[string]any) (Output, error) {
		mu.Lock()
		seen = append(seen, "a")
		mu.Unlock()
		return Main([]Item{{"from": "a"}}), nil
	}})
	reg.Register("b", funcExecutor{desc: NodeDescription{Type: "b"}, execute: func(_ context.Context, _ *Runtime, input []Item, _ map[string]any) (Output, error) {
		mu.Lock()
		seen = append(seen, "b")
		mu.Unlock()
		if len(input) != 1 || input[0]["from"] != "a" {
			t.Fatalf("expected b to receive a's output, got %#v", input)
		}
		return Main(input), nil
	}})

	def := Definition{Name: "test", Nodes: []Node{
		{ID: "a", Type: "a", Connections: map[string][]string{"main": {"b"}}},
		{ID: "b", Type: "b"},
	}}
	result, err := Run(context.Background(), def, reg, nil, nil, Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, results: %#v", result.Results)
	}
	if strings.Join(seen, ",") != "a,b" {
		t.Fatalf("unexpected execution order: %v", seen)
	}
}

func TestRunRoutesConditionalPorts(t *testing.T) {
	reg := NewRegistry()
	reg.Register("split", funcExecutor{desc: NodeDescription{Type: "split"}, execute: func(_ context.Context, _ *Runtime, _ []Item, _ map[string]any) (Output, error) {
		return Output{Ports: map[string][]Item{
			"true":  {{"branch": "true"}},
			"false": {{"branch": "false"}},
		}}, nil
	}})
	var trueGot, falseGot []Item
	reg.Register("sink", funcExecutor{desc: NodeDescription{Type: "sink"}, execute: func(_ context.Context, _ *Runtime, input []Item, params map[string]any) (Output, error) {
		if params["branch"] == "true" {
			trueGot = input
		} else {
			falseGot = input
		}
		return Main(input), nil
	}})

	def := Definition{Nodes: []Node{
		{ID: "split", Type: "split", Connections: map[string][]string{"true": {"sink-true"}, "false": {"sink-false"}}},
		{ID: "sink-true", Type: "sink", Parameters: map[string]any{"branch": "true"}},
		{ID: "sink-false", Type: "sink", Parameters: map[string]any{"branch": "false"}},
	}}
	result, err := Run(context.Background(), def, reg, nil, nil, Options{})
	if err != nil || !result.Success {
		t.Fatalf("run: err=%v success=%v results=%#v", err, result.Success, result.Results)
	}
	if len(trueGot) != 1 || trueGot[0]["branch"] != "true" {
		t.Fatalf("true branch got wrong items: %#v", trueGot)
	}
	if len(falseGot) != 1 || falseGot[0]["branch"] != "false" {
		t.Fatalf("false branch got wrong items: %#v", falseGot)
	}
}

func TestRunSkipsDownstreamOnFailure(t *testing.T) {
	reg := NewRegistry()
	reg.Register("boom", funcExecutor{desc: NodeDescription{Type: "boom"}, execute: func(context.Context, *Runtime, []Item, map[string]any) (Output, error) {
		return Output{}, errors.New("boom")
	}})
	reg.Register("passthrough", passthrough("passthrough"))

	def := Definition{Nodes: []Node{
		{ID: "boom", Type: "boom", Connections: map[string][]string{"main": {"next"}}},
		{ID: "next", Type: "passthrough"},
	}}
	result, err := Run(context.Background(), def, reg, nil, nil, Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Success {
		t.Fatal("expected overall failure")
	}
	if result.Results["boom"].Success {
		t.Fatal("expected boom to fail")
	}
	next := result.Results["next"]
	if !next.Skipped || next.Success {
		t.Fatalf("expected next to be skipped, got %#v", next)
	}
}

func TestValidateDetectsCycle(t *testing.T) {
	def := Definition{Nodes: []Node{
		{ID: "a", Type: "x", Connections: map[string][]string{"main": {"b"}}},
		{ID: "b", Type: "x", Connections: map[string][]string{"main": {"a"}}},
	}}
	if err := Validate(def); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateDetectsDuplicateID(t *testing.T) {
	def := Definition{Nodes: []Node{{ID: "a", Type: "x"}, {ID: "a", Type: "y"}}}
	if err := Validate(def); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestValidateDetectsUnknownConnectionTarget(t *testing.T) {
	def := Definition{Nodes: []Node{
		{ID: "a", Type: "x", Connections: map[string][]string{"main": {"missing"}}},
	}}
	if err := Validate(def); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestRunFailsOnUnregisteredNodeType(t *testing.T) {
	def := Definition{Nodes: []Node{{ID: "a", Type: "does-not-exist"}}}
	result, err := Run(context.Background(), def, NewRegistry(), nil, nil, Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for unregistered node type")
	}
}

func TestRunSeedsStartingNodesWithInput(t *testing.T) {
	reg := NewRegistry()
	reg.Register("start", passthrough("start"))
	def := Definition{Nodes: []Node{{ID: "start", Type: "start"}}}
	result, err := Run(context.Background(), def, reg, nil, []Item{{"seed": true}}, Options{})
	if err != nil || !result.Success {
		t.Fatalf("run: err=%v success=%v", err, result.Success)
	}
	out := result.Results["start"].Output
	if len(out) != 1 || out[0]["seed"] != true {
		t.Fatalf("expected seeded input, got %#v", out)
	}
}
