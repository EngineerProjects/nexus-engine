package engine

import (
	"testing"
	"time"
)

// TestComputeRecoveryBackoffIsExponentialNotLinear is the regression test
// for the fix: attempt N's base delay (before jitter) must double each time
// (250, 500, 1000, 2000...), not grow linearly (250, 500, 750, 1000...) as
// it used to (multiplier*attempt). Jitter only ever adds up to 25% on top,
// so a delay below the previous formula's linear value would prove the
// exponential growth regressed.
func TestComputeRecoveryBackoffIsExponentialNotLinear(t *testing.T) {
	const multiplier = 250 // ms
	// Jitter is random (0-25% extra), so compare against the un-jittered
	// exponential floor for each attempt rather than an exact value.
	floors := []time.Duration{
		250 * time.Millisecond,  // attempt 1: 250 * 2^0
		500 * time.Millisecond,  // attempt 2: 250 * 2^1
		1000 * time.Millisecond, // attempt 3: 250 * 2^2
		2000 * time.Millisecond, // attempt 4: 250 * 2^3
	}
	for i, floor := range floors {
		attempt := i + 1
		delay := computeRecoveryBackoff(attempt, multiplier, 0)
		if delay < floor {
			t.Fatalf("attempt %d: delay = %s, want >= %s (exponential floor, no jitter can go below it)", attempt, delay, floor)
		}
		// Jitter is capped at +25%, so the delay must never exceed 1.25x the floor.
		ceiling := floor + floor/4
		if delay > ceiling {
			t.Fatalf("attempt %d: delay = %s, want <= %s (floor + 25%% jitter cap)", attempt, delay, ceiling)
		}
	}
}

func TestComputeRecoveryBackoffRespectsMaxDelay(t *testing.T) {
	// Attempt 10 would be 250ms * 2^9 = 128s uncapped - must be clamped.
	maxDelayMs := 5000
	delay := computeRecoveryBackoff(10, 250, maxDelayMs)
	ceiling := time.Duration(maxDelayMs) * time.Millisecond * 5 / 4 // +25% jitter
	if delay > ceiling {
		t.Fatalf("delay = %s, want <= %s (capped at MaxDelay + jitter)", delay, ceiling)
	}
	floor := time.Duration(maxDelayMs) * time.Millisecond
	if delay < floor {
		t.Fatalf("delay = %s, want >= %s (capped value itself, before jitter)", delay, floor)
	}
}

func TestComputeRecoveryBackoffDefaultsWhenUnset(t *testing.T) {
	// multiplierMs=0, maxDelayMs=0 must fall back to the documented defaults
	// (250ms base, 30s cap) rather than a zero delay.
	delay := computeRecoveryBackoff(1, 0, 0)
	if delay < 250*time.Millisecond {
		t.Fatalf("delay = %s, want >= 250ms default base for attempt 1", delay)
	}
}

func TestComputeRecoveryBackoffJitterVaries(t *testing.T) {
	// Not a statistical test - just proves jitter is actually applied
	// (not a no-op) by sampling enough draws to see more than one value.
	seen := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		seen[computeRecoveryBackoff(3, 250, 0)] = true
	}
	if len(seen) < 2 {
		t.Fatal("expected jitter to produce varying delays across repeated calls, got a single constant value")
	}
}
