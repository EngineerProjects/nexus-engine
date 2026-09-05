package nodes

import (
	"context"
	"errors"
	"strings"

	"github.com/KPO-Tech/seshat/pkg/dataflow"
)

// WebhookTrigger is the "webhook_trigger" node type: a graph's HTTP entry
// point. It carries the expected HTTP method rather than transforming data -
// the caller that hosts the graph (seshat-server) reads this parameter to
// decide which incoming requests are allowed to start a run, and mints/owns
// the actual URL (a per-job random token, not a user-chosen path - see the
// job-side trigger extraction for why). Execute is a pure passthrough, same
// reasoning as ScheduleTrigger: Run already seeds every node with no
// predecessors (this one included) with the run's input.
type WebhookTrigger struct{}

func NewWebhookTrigger() WebhookTrigger { return WebhookTrigger{} }

var webhookTriggerMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

func (WebhookTrigger) Description() dataflow.NodeDescription {
	options := make([]dataflow.NodePropertyOption, len(webhookTriggerMethods))
	for i, m := range webhookTriggerMethods {
		options[i] = dataflow.NodePropertyOption{Label: m, Value: m}
	}
	return dataflow.NodeDescription{Type: "webhook_trigger", Name: "Webhook Trigger", Category: "Trigger",
		IsTrigger: true,
		Description: "Starts this graph when an external HTTP call hits this job's webhook URL " +
			"(shown once the automation is saved). The request body/headers/query reach the graph " +
			"as this run's input, under a \"webhook\" key.",
		Properties: []dataflow.NodeProperty{
			{Name: "method", DisplayName: "HTTP Method", Type: dataflow.PropOptions, Required: true, Default: "POST",
				Description: "Only requests using this method will trigger a run; others get 405.",
				Options:     options},
		}}
}

func (WebhookTrigger) ValidateParameters(params map[string]any) error {
	method := dataflow.StringParam(params, "method", "")
	if method == "" {
		return nil
	}
	for _, m := range webhookTriggerMethods {
		if strings.EqualFold(method, m) {
			return nil
		}
	}
	return errors.New(`method must be one of "GET", "POST", "PUT", "PATCH", "DELETE"`)
}

func (WebhookTrigger) Execute(_ context.Context, _ *dataflow.Runtime, input []dataflow.Item, _ map[string]any) (dataflow.Output, error) {
	return dataflow.Main(input), nil
}
