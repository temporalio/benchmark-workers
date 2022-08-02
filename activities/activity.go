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
	if heartbeatTimeout == 0 {
		// If no heartbeat timeout is set, assume we don't want heartbeating.
		// Set the heartbeat time longer than our sleep time so that we never send a heartbeat.
		heartbeatTimeout = time.Duration(input.SleepTimeInSeconds) * 2
	}
	heartbeatTick := time.Duration(0.8 * float64(heartbeatTimeout))
	t := time.NewTicker(heartbeatTick)

	for {
		select {
		case <-sleepTimer:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
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
