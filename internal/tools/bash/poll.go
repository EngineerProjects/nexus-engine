package bash

import (
	"context"
	"strings"
	"time"
)

// pollInterval is how often pollForOutput checks for new output/prompt/
// completion. 100ms matches DesktopCommanderMCP's terminal-manager polling
// cadence - fast enough that early-exit actually saves meaningful time
// against typical wait budgets (hundreds of ms to tens of seconds), without
// spinning hard enough to matter for CPU usage.
const pollInterval = 100 * time.Millisecond

// pollForOutput polls a background task's output until one of three things
// happens, whichever comes first:
//  1. the accumulated output looks like it ends at an interactive prompt
//     (looksLikeAwaitingInput) - the process is almost certainly idle,
//     waiting for input, so there's nothing to gain by waiting out the rest
//     of maxWait;
//  2. the task finishes (completed/killed/timed out);
//  3. maxWait elapses.
//
// This replaces a single fixed-duration wait with early-exit polling: a fast
// REPL reply returns almost immediately instead of always paying the full
// wait_ms/timeout_ms, and a slow one still gets the full budget instead of
// coming back with an incomplete reply.
func pollForOutput(ctx context.Context, mgr *BackgroundTaskManager, taskID string, reader *TaskOutputReader, maxWait time.Duration) (output string, status TaskStatus) {
	deadline := time.Now().Add(maxWait)
	var accumulated strings.Builder

	for {
		if chunk, err := reader.ReadOutput(); err == nil && chunk != "" {
			accumulated.WriteString(chunk)
		}

		status = TaskStatusRunning
		if task := mgr.GetTask(taskID); task != nil {
			status = task.GetStatus()
		}
		if status != TaskStatusRunning && status != TaskStatusBackgrounded {
			return accumulated.String(), status
		}
		if accumulated.Len() > 0 && looksLikeAwaitingInput(accumulated.String()) {
			return accumulated.String(), status
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return accumulated.String(), status
		}
		wait := pollInterval
		if remaining < wait {
			wait = remaining
		}
		select {
		case <-ctx.Done():
			return accumulated.String(), status
		case <-time.After(wait):
		}
	}
}
