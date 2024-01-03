package workflows

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

type ExecuteActivityWorkflowInput struct {
	Count    int
	Activity string
	Input    interface{}
}

type ReceiveSignalWorkflowInput struct {
	Count int
	Name  string
}

func ExecuteActivityWorkflow(ctx workflow.Context, input ExecuteActivityWorkflowInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	for i := 0; i < input.Count; i++ {
		err := workflow.ExecuteActivity(ctx, input.Activity, input.Input).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func ReceiveSignalWorkflow(ctx workflow.Context, input ReceiveSignalWorkflowInput) error {
	ch := workflow.GetSignalChannel(ctx, input.Name)

	for i := 0; i < input.Count; i++ {
		var data interface{}

		ch.Receive(ctx, &data)
	}

	return nil
}
