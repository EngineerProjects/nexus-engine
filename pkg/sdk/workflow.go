package sdk

import (
	"context"

	"github.com/KPO-Tech/seshat/pkg/workflow"
)

type WorkflowDefinition = workflow.Definition
type WorkflowNode = workflow.Node
type WorkflowResult = workflow.Result
type WorkflowNodeResult = workflow.NodeResult
type WorkflowDraftOptions = workflow.DraftOptions
type WorkflowDraftResult = workflow.DraftResult

type WorkflowOptions struct {
	MaxParallel int
	Tools       []Tool
	Executor    workflow.Executor
}

func LoadWorkflowFile(path string) (WorkflowDefinition, error) {
	return workflow.LoadFile(path)
}

func ValidateWorkflow(def WorkflowDefinition) error {
	return workflow.Validate(def)
}

func DraftWorkflow(options WorkflowDraftOptions) (WorkflowDraftResult, error) {
	return workflow.Draft(options)
}

func BuildWorkflowNodePrompt(node WorkflowNode, inputs map[string]WorkflowNodeResult) string {
	return workflow.BuildNodePrompt(node, inputs)
}

func (c *Client) RunWorkflow(ctx context.Context, def WorkflowDefinition, options WorkflowOptions) (WorkflowResult, error) {
	executor := options.Executor
	if executor == nil {
		executor = workflow.ExecutorFunc(func(execCtx context.Context, node workflow.Node, inputs map[string]workflow.NodeResult) (string, error) {
			response, err := c.Ask(execCtx, workflow.BuildNodePrompt(node, inputs), options.Tools)
			if err != nil {
				return "", err
			}
			return response.Content, nil
		})
	}
	return workflow.Run(ctx, def, executor, workflow.Options{MaxParallel: options.MaxParallel})
}
