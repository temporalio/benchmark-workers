package workflows

import (
	"time"

	"github.com/temporalio/benchmark-workers/activities"
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

// DSL step: either an activity or a child workflow (which is always this workflow)
type DSLStep struct {
	Activity    string      `json:"a,omitempty"`
	Input       interface{} `json:"i,omitempty"`
	Child       []DSLStep   `json:"c,omitempty"`
	Repeat      int         `json:"r,omitempty"`
	PaddingSize int         `json:"p,omitempty"` // Size in bytes of padding to add to activity inputs
}

// injectPadding adds padding data to an activity input using the Paddable interface
func injectPadding(input interface{}, paddingSize int) interface{} {
	if paddingSize <= 0 {
		return input
	}

	// Create padding data
	padding := make([]byte, paddingSize)
	for i := range padding {
		padding[i] = byte(i % 256) // Fill with repeating byte pattern
	}

	// Use the clean interface-based approach
	if paddable, ok := input.(activities.Paddable); ok {
		paddable.SetPadding(padding)
	}

	return input
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

// DSLWorkflow executes a list of DSLStep instructions.
func DSLWorkflow(ctx workflow.Context, steps []DSLStep) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	for _, step := range steps {
		repeat := step.Repeat
		if repeat <= 0 {
			repeat = 1
		}
		for i := 0; i < repeat; i++ {
			if step.Activity != "" {
				// Inject padding into the activity input if specified
				activityInput := injectPadding(step.Input, step.PaddingSize)
				if err := workflow.ExecuteActivity(ctx, step.Activity, activityInput).Get(ctx, nil); err != nil {
					return err
				}
			}
			if len(step.Child) > 0 {
				if err := workflow.ExecuteChildWorkflow(ctx, DSLWorkflow, step.Child).Get(ctx, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
