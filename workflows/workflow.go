package workflows

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

func ExecuteActivityWorkflow(ctx workflow.Context, rounds int, activity string, inputs ...interface{}) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	for i := 0; i < rounds; i++ {
		err := workflow.ExecuteActivity(ctx, activity, inputs...).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
