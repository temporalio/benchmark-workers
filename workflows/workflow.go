package workflows

import (
	"time"

	"github.com/robholland/benchmark-workers/activities"
	"go.temporal.io/sdk/workflow"
)

func SerialWorkflow(ctx workflow.Context, input string, rounds int, delay int) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string

	for i := 0; i < rounds; i++ {
		err := workflow.ExecuteActivity(ctx, activities.SleepActivity, input, delay).Get(ctx, &result)
		if err != nil {
			return "", err
		}
	}

	return result, nil
}
