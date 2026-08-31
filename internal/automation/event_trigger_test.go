package automation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeJobStore is a minimal in-memory JobStore for tests that don't need a
// real database - just enough for AddJob/RunEvent's own bookkeeping calls.
type fakeJobStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
	runs map[string]*JobRun
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{jobs: make(map[string]*Job), runs: make(map[string]*JobRun)}
}

func (s *fakeJobStore) CreateJob(_ context.Context, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *job
	s.jobs[job.ID] = &cp
	return nil
}

func (s *fakeJobStore) GetJob(_ context.Context, id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, nil
	}
	cp := *job
	return &cp, nil
}

func (s *fakeJobStore) ListJobs(_ context.Context, ownerID string) ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Job
	for _, job := range s.jobs {
		if ownerID == "" || job.OwnerID == ownerID {
			cp := *job
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *fakeJobStore) UpdateJob(_ context.Context, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *job
	s.jobs[job.ID] = &cp
	return nil
}

func (s *fakeJobStore) DeleteJob(_ context.Context, id, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return nil
}

func (s *fakeJobStore) CreateRun(_ context.Context, run *JobRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *run
	s.runs[run.ID] = &cp
	return nil
}

func (s *fakeJobStore) UpdateRun(_ context.Context, run *JobRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *run
	s.runs[run.ID] = &cp
	return nil
}

func (s *fakeJobStore) ListRuns(_ context.Context, jobID string, _ int) ([]*JobRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*JobRun
	for _, run := range s.runs {
		if run.JobID == jobID {
			cp := *run
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *fakeJobStore) GetRun(_ context.Context, id string) (*JobRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return nil, nil
	}
	cp := *run
	return &cp, nil
}

func TestTriggerIsEvent(t *testing.T) {
	if (Trigger{Type: TriggerTypeCron}).IsEvent() {
		t.Error("a cron trigger should not report IsEvent")
	}
	if !(Trigger{Type: TriggerTypeEvent}).IsEvent() {
		t.Error("an event trigger should report IsEvent")
	}
}

func TestTriggerToScheduleRejectsEvent(t *testing.T) {
	_, err := (Trigger{Type: TriggerTypeEvent, EventType: "inbox.message.received"}).ToSchedule()
	if err == nil {
		t.Fatal("expected ToSchedule to reject a TriggerTypeEvent trigger - callers must check IsEvent first")
	}
}

func TestAddJobEventTriggerHasNoNextRunAt(t *testing.T) {
	store := newFakeJobStore()
	sched := NewJobScheduler(store, nil)

	job := &Job{
		Name: "reply to invoices",
		Trigger: Trigger{
			Type:      TriggerTypeEvent,
			EventType: "inbox.message.received",
		},
		Task: "draft a reply",
	}
	if err := sched.AddJob(context.Background(), job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	if job.NextRunAt != nil {
		t.Fatalf("expected an event job to have no NextRunAt, got %v", job.NextRunAt)
	}
	if job.Status != JobStatusActive {
		t.Fatalf("Status = %q, want %q", job.Status, JobStatusActive)
	}

	stored, err := store.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if stored == nil {
		t.Fatal("expected the event job to be persisted")
	}
	if stored.Trigger.EventType != "inbox.message.received" {
		t.Fatalf("stored Trigger.EventType = %q, want %q", stored.Trigger.EventType, "inbox.message.received")
	}
}

func TestApplyRunOutcomeEventTriggerStaysActiveWithNoNextRunAt(t *testing.T) {
	job := &Job{
		Trigger: Trigger{Type: TriggerTypeEvent, EventType: "inbox.message.received"},
		Status:  JobStatusActive,
	}
	run := &JobRun{Status: RunStatusSuccess}

	applyRunOutcome(job, run, time.Now())

	if job.RunCount != 1 {
		t.Fatalf("RunCount = %d, want 1", job.RunCount)
	}
	if job.Status != JobStatusActive {
		t.Fatalf("Status = %q, want %q (an event job stays active indefinitely, no schedule to exhaust)", job.Status, JobStatusActive)
	}
	if job.NextRunAt != nil {
		t.Fatalf("NextRunAt = %v, want nil (event jobs are never schedule-driven)", job.NextRunAt)
	}
}

func TestRunEventRecordsRunOnResolverError(t *testing.T) {
	store := newFakeJobStore()
	sched := NewJobScheduler(store, nil)
	sched.SetRunnerResolver(func(_ context.Context, _, _, _ string) (*Runner, AgentConfig, error) {
		return nil, AgentConfig{}, errors.New("no credentials configured")
	})

	job := &Job{
		ID:      "job-1",
		Trigger: Trigger{Type: TriggerTypeEvent, EventType: "inbox.message.received"},
		Task:    "draft a reply",
	}
	if err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("seed CreateJob: %v", err)
	}

	run, err := sched.RunEvent(context.Background(), job, "Triggering message:\nFrom: someone@example.com")
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	if run.Status != RunStatusError {
		t.Fatalf("run.Status = %q, want %q", run.Status, RunStatusError)
	}
	if run.Error == "" {
		t.Fatal("expected run.Error to explain the resolver failure")
	}

	stored, err := store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if stored == nil || stored.Status != RunStatusError {
		t.Fatalf("expected the run to be persisted with RunStatusError, got %+v", stored)
	}
}
