package activities

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

func SleepActivity(ctx context.Context, sleepTimeInSeconds int) error {
	sleepTimer := time.After(time.Duration(sleepTimeInSeconds) * time.Second)
	heartbeatTimeout := activity.GetInfo(ctx).HeartbeatTimeout
	if heartbeatTimeout == 0 {
		// If no heartbeat timeout is set, assume we don't want heartbeating.
		// Set the heartbeat time longer than our sleep time so that we never send a heartbeat.
		heartbeatTimeout = time.Duration(sleepTimeInSeconds) * 2
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

func EchoActivity(ctx context.Context, input string) (string, error) {
	return input, nil
}
