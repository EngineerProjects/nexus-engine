package nodes

import (
	"context"
	"errors"
	"time"

	"github.com/KPO-Tech/seshat/pkg/dataflow"
)

// maxWait bounds the "wait" node's seconds parameter — a graph running
// inside a Job (see internal/automation.Job.MaxDuration) shouldn't be able
// to block a worker indefinitely via a huge or negative wait value.
const maxWait = 10 * time.Minute

// Wait is the "wait" node type: pauses before passing items through
// unchanged. Parameters: seconds (int, required, 0 < seconds <= 600).
type Wait struct{}

func NewWait() Wait { return Wait{} }

func (Wait) Description() dataflow.NodeDescription {
	return dataflow.NodeDescription{Type: "wait", Name: "Wait", Category: "Control",
		Description: "Pauses for a fixed duration, then passes items through unchanged. Blocks the whole graph run for that long — keep it short relative to the job's MaxDuration, if one is set. " +
			"Parameters: seconds (integer, required, 1-600)."}
}

func (Wait) ValidateParameters(params map[string]any) error {
	seconds := dataflow.IntParam(params, "seconds", 0)
	if seconds <= 0 {
		return errors.New("seconds must be a positive integer")
	}
	if time.Duration(seconds)*time.Second > maxWait {
		return errors.New("seconds exceeds the 600s maximum")
	}
	return nil
}

func (Wait) Execute(ctx context.Context, _ *dataflow.Runtime, input []dataflow.Item, params map[string]any) (dataflow.Output, error) {
	seconds := dataflow.IntParam(params, "seconds", 0)
	select {
	case <-time.After(time.Duration(seconds) * time.Second):
	case <-ctx.Done():
		return dataflow.Output{}, ctx.Err()
	}
	return dataflow.Main(input), nil
}
