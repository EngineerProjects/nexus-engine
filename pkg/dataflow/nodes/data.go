package nodes

import (
	"context"
	"strings"

	"github.com/KPO-Tech/seshat/pkg/dataflow"
	"github.com/KPO-Tech/seshat/pkg/dataflow/expr"
)

// Set is the "set" node type: adds/overwrites fields on every item.
// Parameters: fields (map[string]any) — a string value starting with "="
// (n8n's convention for "this is an expression, not a literal") is
// evaluated per item with $json bound to the item being modified;
// everything else is copied through as a literal.
type Set struct{ pool *expr.Pool }

func NewSet(pool *expr.Pool) *Set { return &Set{pool: pool} }

func (n *Set) Description() dataflow.NodeDescription {
	return dataflow.NodeDescription{Type: "set", Name: "Set", Category: "Data",
		Description: "Adds or overwrites fields on every item, keeping existing fields not mentioned. " +
			"Parameters: fields (object, required) — field name -> value; a string value starting with \"=\" is evaluated as a JS expression (current item bound as $json, e.g. \"=$json.n * 2\"), anything else (including a plain string) is used as a literal."}
}

func (n *Set) ValidateParameters(map[string]any) error { return nil }

func (n *Set) Execute(ctx context.Context, rt *dataflow.Runtime, input []dataflow.Item, params map[string]any) (dataflow.Output, error) {
	fields, _ := params["fields"].(map[string]any)
	out := make([]dataflow.Item, 0, len(input))
	for _, item := range input {
		next := make(dataflow.Item, len(item)+len(fields))
		for k, v := range item {
			next[k] = v
		}
		for k, v := range fields {
			if expression, ok := v.(string); ok && strings.HasPrefix(expression, "=") {
				value, err := n.pool.Eval(ctx, strings.TrimPrefix(expression, "="), map[string]any{"$json": map[string]any(item)})
				if err != nil {
					return dataflow.Output{}, err
				}
				next[k] = value
				continue
			}
			next[k] = v
		}
		out = append(out, next)
	}
	return dataflow.Main(out), nil
}

// Merge is the "merge" node type. The engine already concatenates items
// from every upstream connection into one input slice (see engine.go), so
// by default this is an identity pass-through — its value is as an explicit,
// labeled join point in a graph rather than as a transformation. Fancier
// merge strategies (pair-by-index, join-by-key) are deliberately not built
// yet; add them here if/when a workflow actually needs one.
type Merge struct{}

func NewMerge() Merge { return Merge{} }

func (Merge) Description() dataflow.NodeDescription {
	return dataflow.NodeDescription{Type: "merge", Name: "Merge", Category: "Data",
		Description: "Joins items from multiple upstream branches into one list — connect two or more nodes' outputs to this node's id to combine them; it needs no parameters of its own, the engine already concatenates every input a node receives. Use it as an explicit join point when a graph branches then needs to reconverge. " +
			"Parameters: none."}
}

func (Merge) ValidateParameters(map[string]any) error { return nil }

func (Merge) Execute(_ context.Context, _ *dataflow.Runtime, input []dataflow.Item, _ map[string]any) (dataflow.Output, error) {
	return dataflow.Main(input), nil
}
