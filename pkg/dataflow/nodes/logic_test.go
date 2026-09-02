package nodes

import (
	"context"
	"testing"

	"github.com/KPO-Tech/seshat/pkg/dataflow"
	"github.com/KPO-Tech/seshat/pkg/dataflow/expr"
)

func TestFilterKeepsMatchingItems(t *testing.T) {
	n := NewFilter(expr.NewPool(2))
	out, err := n.Execute(context.Background(), nil, []dataflow.Item{{"n": 1}, {"n": 5}, {"n": 10}}, map[string]any{"expression": "$json.n > 3"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	kept := out.Ports["main"]
	if len(kept) != 2 || kept[0]["n"] != 5 || kept[1]["n"] != 10 {
		t.Fatalf("unexpected kept items: %#v", kept)
	}
}

func TestIfSplitsIntoTrueFalsePorts(t *testing.T) {
	n := NewIf(expr.NewPool(2))
	out, err := n.Execute(context.Background(), nil, []dataflow.Item{{"n": 1}, {"n": 9}}, map[string]any{"expression": "$json.n > 5"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(out.Ports["true"]) != 1 || out.Ports["true"][0]["n"] != 9 {
		t.Fatalf("unexpected true branch: %#v", out.Ports["true"])
	}
	if len(out.Ports["false"]) != 1 || out.Ports["false"][0]["n"] != 1 {
		t.Fatalf("unexpected false branch: %#v", out.Ports["false"])
	}
}

func TestSwitchRoutesFirstMatchingCase(t *testing.T) {
	n := NewSwitch(expr.NewPool(2))
	params := map[string]any{
		"casesOrder": []any{"low", "high"},
		"cases": map[string]any{
			"low":  "$json.n < 5",
			"high": "$json.n >= 5",
		},
	}
	out, err := n.Execute(context.Background(), nil, []dataflow.Item{{"n": 1}, {"n": 9}, {"n": 100}}, params)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(out.Ports["low"]) != 1 || out.Ports["low"][0]["n"] != 1 {
		t.Fatalf("unexpected low case: %#v", out.Ports["low"])
	}
	if len(out.Ports["high"]) != 2 {
		t.Fatalf("unexpected high case: %#v", out.Ports["high"])
	}
}

func TestSwitchFallsBackToDefaultPort(t *testing.T) {
	n := NewSwitch(expr.NewPool(2))
	params := map[string]any{
		"casesOrder": []any{"never"},
		"cases":      map[string]any{"never": "false"},
	}
	out, err := n.Execute(context.Background(), nil, []dataflow.Item{{"n": 1}}, params)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(out.Ports["default"]) != 1 {
		t.Fatalf("expected item on default port, got %#v", out.Ports)
	}
}

func TestSwitchValidateParametersRequiresKnownCases(t *testing.T) {
	n := NewSwitch(expr.NewPool(2))
	err := n.ValidateParameters(map[string]any{
		"casesOrder": []any{"missing"},
		"cases":      map[string]any{"other": "true"},
	})
	if err == nil {
		t.Fatal("expected error for casesOrder referencing an unknown case")
	}
}
