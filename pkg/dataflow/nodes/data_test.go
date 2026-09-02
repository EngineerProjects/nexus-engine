package nodes

import (
	"context"
	"testing"

	"github.com/KPO-Tech/seshat/pkg/dataflow"
	"github.com/KPO-Tech/seshat/pkg/dataflow/expr"
)

func TestSetAddsLiteralAndExpressionFields(t *testing.T) {
	n := NewSet(expr.NewPool(2))
	out, err := n.Execute(context.Background(), nil, []dataflow.Item{{"n": 3}}, map[string]any{
		"fields": map[string]any{
			"label":  "constant",
			"double": "=$json.n * 2",
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	item := out.Ports["main"][0]
	if item["n"] != 3 {
		t.Fatalf("expected original field preserved, got %#v", item)
	}
	if item["label"] != "constant" {
		t.Fatalf("expected literal field, got %#v", item["label"])
	}
	if got := toFloat(t, item["double"]); got != 6 {
		t.Fatalf("expected evaluated field 6, got %#v (%T)", item["double"], item["double"])
	}
}

func toFloat(t *testing.T, v any) float64 {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return float64(n)
	case float64:
		return n
	case int:
		return float64(n)
	default:
		t.Fatalf("expected a numeric value, got %#v (%T)", v, v)
		return 0
	}
}

func TestMergeIsIdentityPassthrough(t *testing.T) {
	n := NewMerge()
	input := []dataflow.Item{{"a": 1}, {"b": 2}}
	out, err := n.Execute(context.Background(), nil, input, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(out.Ports["main"]) != 2 {
		t.Fatalf("expected pass-through of both items, got %#v", out.Ports["main"])
	}
}
