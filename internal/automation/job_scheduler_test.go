package automation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFormatExecErrorTimeout(t *testing.T) {
	got := formatExecError(context.DeadlineExceeded, 5*time.Second)
	want := "execution timed out after 5s"
	if got != want {
		t.Fatalf("formatExecError = %q, want %q", got, want)
	}
}

func TestFormatExecErrorWrappedTimeout(t *testing.T) {
	wrapped := errors.New("wrapped: " + context.DeadlineExceeded.Error())
	// errors.Is only matches an actual wrapped chain, not a string that
	// merely contains the message - this proves formatExecError uses
	// errors.Is (not string matching) and falls through correctly.
	got := formatExecError(wrapped, time.Second)
	if got != wrapped.Error() {
		t.Fatalf("formatExecError = %q, want the original error message %q (not misdetected as a timeout)", got, wrapped.Error())
	}
}

func TestFormatExecErrorGeneric(t *testing.T) {
	err := errors.New("boom")
	got := formatExecError(err, time.Minute)
	if got != "boom" {
		t.Fatalf("formatExecError = %q, want %q", got, "boom")
	}
}

func TestApplyRunOutcomeReschedulesOnSuccess(t *testing.T) {
	job := &Job{
		Trigger:  Trigger{Type: TriggerTypeInterval, Interval: time.Hour},
		Status:   JobStatusActive,
		RunCount: 0,
	}
	run := &JobRun{Status: RunStatusSuccess}
	endedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	applyRunOutcome(job, run, endedAt)

	if job.RunCount != 1 {
		t.Fatalf("RunCount = %d, want 1", job.RunCount)
	}
	if job.Status != JobStatusActive {
		t.Fatalf("Status = %q, want %q", job.Status, JobStatusActive)
	}
	if job.NextRunAt == nil || !job.NextRunAt.Equal(endedAt.Add(time.Hour)) {
		t.Fatalf("NextRunAt = %v, want %v", job.NextRunAt, endedAt.Add(time.Hour))
	}
	if job.LastRunStatus != string(RunStatusSuccess) {
		t.Fatalf("LastRunStatus = %q, want %q", job.LastRunStatus, RunStatusSuccess)
	}
}

// TestApplyRunOutcomeStopsAtMaxRuns is the regression test for the MaxRuns
// budget: once RunCount reaches MaxRuns, the job goes Inactive instead of
// being rescheduled again, even though its interval trigger would otherwise
// keep firing forever.
func TestApplyRunOutcomeStopsAtMaxRuns(t *testing.T) {
	job := &Job{
		Trigger:  Trigger{Type: TriggerTypeInterval, Interval: time.Hour},
		Status:   JobStatusActive,
		MaxRuns:  3,
		RunCount: 2, // this run is the 3rd
	}
	run := &JobRun{Status: RunStatusSuccess}
	endedAt := time.Now()

	applyRunOutcome(job, run, endedAt)

	if job.RunCount != 3 {
		t.Fatalf("RunCount = %d, want 3", job.RunCount)
	}
	if job.Status != JobStatusInactive {
		t.Fatalf("Status = %q, want %q once MaxRuns is reached", job.Status, JobStatusInactive)
	}
	if job.NextRunAt != nil {
		t.Fatalf("NextRunAt = %v, want nil once the job goes Inactive", job.NextRunAt)
	}
}

func TestApplyRunOutcomeUnderMaxRunsStillReschedules(t *testing.T) {
	job := &Job{
		Trigger:  Trigger{Type: TriggerTypeInterval, Interval: time.Hour},
		Status:   JobStatusActive,
		MaxRuns:  3,
		RunCount: 1, // this run is only the 2nd
	}
	run := &JobRun{Status: RunStatusSuccess}
	endedAt := time.Now()

	applyRunOutcome(job, run, endedAt)

	if job.Status != JobStatusActive {
		t.Fatalf("Status = %q, want %q (budget not yet exhausted)", job.Status, JobStatusActive)
	}
	if job.NextRunAt == nil {
		t.Fatal("expected NextRunAt to be set (budget not yet exhausted)")
	}
}

func TestApplyRunOutcomeOnceTriggerGoesInactive(t *testing.T) {
	runAt := time.Now().Add(-time.Minute) // already in the past relative to endedAt below
	job := &Job{
		Trigger: Trigger{Type: TriggerTypeOnce, RunAt: &runAt},
		Status:  JobStatusActive,
	}
	run := &JobRun{Status: RunStatusSuccess}

	applyRunOutcome(job, run, time.Now())

	if job.Status != JobStatusInactive {
		t.Fatalf("Status = %q, want %q for an exhausted once-trigger", job.Status, JobStatusInactive)
	}
}
