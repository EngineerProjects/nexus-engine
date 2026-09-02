package automation

import (
	"testing"
	"time"
)

func TestCronParsesBasicFields(t *testing.T) {
	sched, err := Cron("30 9 * * *") // 9:30am every day
	if err != nil {
		t.Fatalf("Cron: %v", err)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next := sched.Next(from)
	want := time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("Next(%s) = %s, want %s", from, next, want)
	}
}

func TestCronStepAndRange(t *testing.T) {
	// Every 15 minutes between 09:00 and 09:45.
	sched, err := Cron("0-45/15 9 * * *")
	if err != nil {
		t.Fatalf("Cron: %v", err)
	}
	from := time.Date(2026, 1, 1, 9, 5, 0, 0, time.UTC)
	next := sched.Next(from)
	want := time.Date(2026, 1, 1, 9, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("Next(%s) = %s, want %s", from, next, want)
	}
}

func TestCronCommaList(t *testing.T) {
	sched, err := Cron("0 8,12,18 * * *")
	if err != nil {
		t.Fatalf("Cron: %v", err)
	}
	from := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	next := sched.Next(from)
	want := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("Next(%s) = %s, want %s", from, next, want)
	}
}

// TestCronDayOfMonthAndWeekdayAreOred is the regression test for the POSIX
// cron semantics bug: when both dom and dow are restricted (neither is *),
// a match on EITHER field fires the job, not only when both coincide. The
// previous hand-rolled parser required both to match (AND), so "0 9 15 * 1"
// only fired when the 15th happened to be a Monday. 2026-01-05 is a Monday
// that is not the 15th - the fix must still fire on it.
func TestCronDayOfMonthAndWeekdayAreOred(t *testing.T) {
	sched, err := Cron("0 9 15 * 1") // 9am on the 15th, and every Monday
	if err != nil {
		t.Fatalf("Cron: %v", err)
	}

	// 2026-01-01 is a Thursday; the next Monday (2026-01-05) is not the 15th.
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next := sched.Next(from)
	wantMonday := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	if !next.Equal(wantMonday) {
		t.Fatalf("Next(%s) = %s, want the Monday-only match %s (dom/dow must be ORed, not ANDed)", from, next, wantMonday)
	}

	// Fires on the 15th itself too, even though 2026-01-15 is a Thursday -
	// not just on Mondays. The next Monday after 2026-01-05 is 2026-01-12,
	// which still isn't the 15th, so start the search just after that to
	// isolate the dom-only match.
	afterSecondMonday := time.Date(2026, 1, 12, 9, 1, 0, 0, time.UTC)
	next2 := sched.Next(afterSecondMonday)
	want15th := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	if !next2.Equal(want15th) {
		t.Fatalf("Next(%s) = %s, want the 15th-only match %s (dom/dow must be ORed, not ANDed)", afterSecondMonday, next2, want15th)
	}
}

func TestIntervalSchedule(t *testing.T) {
	sched := Every(30 * time.Minute)
	from := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	next := sched.Next(from)
	want := time.Date(2026, 1, 1, 9, 30, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("Next(%s) = %s, want %s", from, next, want)
	}
}

func TestOnceScheduleFiresOnlyIfInFuture(t *testing.T) {
	at := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	sched := Once(at)

	before := at.Add(-time.Hour)
	if next := sched.Next(before); !next.Equal(at) {
		t.Fatalf("Next(%s) = %s, want %s", before, next, at)
	}

	after := at.Add(time.Hour)
	if next := sched.Next(after); !next.IsZero() {
		t.Fatalf("Next(%s) = %s, want zero time (already fired)", after, next)
	}
}
