package activities

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

type SleepActivityInput struct {
	SleepTimeInSeconds int
}

func SleepActivity(ctx context.Context, input SleepActivityInput) error {
	sleepTimer := time.After(time.Duration(input.SleepTimeInSeconds) * time.Second)
	heartbeatTimeout := activity.GetInfo(ctx).HeartbeatTimeout

	// If no heartbeat timeout is set, we don't need heartbeating
	if heartbeatTimeout == 0 {
		for {
			select {
			case <-sleepTimer:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Create ticker only when heartbeating is needed
	heartbeatTick := time.Duration(0.8 * float64(heartbeatTimeout))
	ticker := time.NewTicker(heartbeatTick)
	defer ticker.Stop()

	for {
		select {
		case <-sleepTimer:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			activity.RecordHeartbeat(ctx)
		}
	}
}

type EchoActivityInput struct {
	Message string
}

func EchoActivity(ctx context.Context, input EchoActivityInput) (string, error) {
	return input.Message, nil
}
