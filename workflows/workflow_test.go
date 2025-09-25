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

func TestDSLWorkflowWithPadding(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	env.RegisterActivityWithOptions(activities.EchoActivity, activity.RegisterOptions{Name: "Echo"})
	env.RegisterActivityWithOptions(activities.SleepActivity, activity.RegisterOptions{Name: "Sleep"})

	env.OnActivity("Echo", mock.Anything, mock.Anything).Return(func(ctx context.Context, input activities.EchoActivityInput) (string, error) {
		return input.Message, nil
	})

	env.OnActivity("Sleep", mock.Anything, mock.Anything).Return(func(ctx context.Context, input activities.SleepActivityInput) error {
		return nil
	})

	steps := []DSLStep{
		{
			Activity:    "Echo",
			Input:       &activities.EchoActivityInput{Message: "test"},
			PaddingSize: 100, // 100 bytes of padding
		},
		{
			Activity:    "Sleep",
			Input:       &activities.SleepActivityInput{SleepTimeInSeconds: 1},
			PaddingSize: 50, // 50 bytes of padding
		},
		{
			Activity: "Echo",
			Input:    &activities.EchoActivityInput{Message: "no padding"},
			// No PaddingSize specified
		},
	}

	env.ExecuteWorkflow(DSLWorkflow, steps)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// The DSL workflow completed successfully - padding injection works at execution time
	// Individual padding verification is done in TestInjectPadding
}

func TestInjectPadding(t *testing.T) {
	// Test with EchoActivityInput
	echoInput := &activities.EchoActivityInput{Message: "test"}
	result := injectPadding(echoInput, 100)

	// Should return the same pointer
	require.Same(t, echoInput, result)
	// Should have padding set
	require.Len(t, echoInput.Padding, 100, "Should have 100 bytes of padding")

	// Test with SleepActivityInput
	sleepInput := &activities.SleepActivityInput{SleepTimeInSeconds: 5}
	result = injectPadding(sleepInput, 50)

	// Should return the same pointer
	require.Same(t, sleepInput, result)
	// Should have padding set
	require.Len(t, sleepInput.Padding, 50, "Should have 50 bytes of padding")

	// Test with zero padding
	echoInputNoPadding := &activities.EchoActivityInput{Message: "no padding"}
	result = injectPadding(echoInputNoPadding, 0)

	// Should return the same pointer
	require.Same(t, echoInputNoPadding, result)
	// Should have no padding
	require.Nil(t, echoInputNoPadding.Padding, "Should have no padding")

	// Test with non-Paddable input (should be returned unchanged)
	nonPaddable := "not paddable"
	result = injectPadding(nonPaddable, 100)
	require.Equal(t, nonPaddable, result)
}
