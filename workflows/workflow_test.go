package workflows

import (
	"context"
	"testing"

	"github.com/temporalio/benchmark-workers/activities"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestDSLWorkflow(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	env.RegisterActivityWithOptions(activities.EchoActivity, activity.RegisterOptions{Name: "Echo"})
	var echoCount int
	env.OnActivity("Echo", mock.Anything, mock.Anything).Return(func(ctx context.Context, input activities.EchoActivityInput) (string, error) {
		echoCount++
		return input.Message, nil
	})

	steps := []DSLStep{
		{Activity: "Echo", Input: map[string]interface{}{"Message": "test"}, Repeat: 3},
		{Child: []DSLStep{
			{Activity: "Echo", Input: map[string]interface{}{"Message": "test"}, Repeat: 3},
		}},
	}

	env.ExecuteWorkflow(DSLWorkflow, steps)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 6, echoCount, "Echo activity should be called 6 times")
}
