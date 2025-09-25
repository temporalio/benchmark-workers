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
			Input:       map[string]interface{}{"Message": "test"},
			PaddingSize: 100, // 100 bytes of padding
		},
		{
			Activity:    "Sleep",
			Input:       map[string]interface{}{"SleepTimeInSeconds": 1},
			PaddingSize: 50, // 50 bytes of padding
		},
		{
			Activity: "Echo",
			Input:    map[string]interface{}{"Message": "no padding"},
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
	// Test with map input (simulating JSON deserialization)
	echoInput := map[string]interface{}{"Message": "test"}
	injectPadding(echoInput, 100)

	// Should have padding added
	require.Contains(t, echoInput, "Padding", "Should have Padding field added")
	require.Len(t, echoInput["Padding"], 100, "Should have 100 bytes of padding")

	// Test with sleep activity input
	sleepInput := map[string]interface{}{"SleepTimeInSeconds": 5}
	injectPadding(sleepInput, 50)

	// Should have padding added
	require.Contains(t, sleepInput, "Padding", "Should have Padding field added")
	require.Len(t, sleepInput["Padding"], 50, "Should have 50 bytes of padding")

	// Test with zero padding
	echoInputNoPadding := map[string]interface{}{"Message": "no padding"}
	injectPadding(echoInputNoPadding, 0)

	// Should not have padding added
	require.NotContains(t, echoInputNoPadding, "Padding", "Should not have Padding field added")

	// Test with non-map input (should be unchanged - no panic)
	nonMap := "not a map"
	require.NotPanics(t, func() {
		injectPadding(nonMap, 100)
	}, "Should not panic with non-map input")
}
