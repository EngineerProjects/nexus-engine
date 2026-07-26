// Package auto - Telemetry and logging for auto mode.
//
// This module provides telemetry and logging support for the classifier,
// tracking classification outcomes, errors, and performance metrics.
// Currently outputs to standard log; can be extended for external telemetry.
package auto

import (
	"encoding/json"
	"fmt"
	"log"
)

// AutoModeOutcome represents a single classification outcome for telemetry.
// Tracks the result of each classification request for monitoring and debugging.
type AutoModeOutcome struct {
	Outcome    string         `json:"outcome"`     // Outcome type (success/error/parse_failure)
	Model      string         `json:"model"`       // Model used for classification
	DurationMs int64          `json:"duration_ms"` // Elapsed time of the classification call
	Metadata   map[string]any `json:"metadata"`    // Additional outcome metadata
}

// durationMs must be the elapsed time of the classification call
// (time.Since(start).Milliseconds()), not a timestamp - this used to be
// time.Now().UnixMilli(), which produced a "duration" that was really just
// "now", off by decades from any real elapsed time.
func LogAutoModeOutcome(outcome string, model string, durationMs int64, metadata map[string]any) {
	entry := AutoModeOutcome{
		Outcome:    outcome,
		Model:      model,
		DurationMs: durationMs,
		Metadata:   metadata,
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[auto-mode] failed to marshal telemetry: %v", err)
		return
	}

	log.Printf("[auto-mode] outcome=%s %s", outcome, string(jsonBytes))
}

const (
	OutcomeSuccess           = "success"
	OutcomeParseFailure      = "parse_failure"
	OutcomeInterrupted       = "interrupted"
	OutcomeTranscriptTooLong = "transcript_too_long"
	OutcomeError             = "error"
)

func LogSuccess(model string, durationMs int64) {
	LogAutoModeOutcome(OutcomeSuccess, model, durationMs, map[string]any{})
}

// Currently unreferenced - no call site tracks elapsed time for a parse
// failure, so 0 here isn't a real duration either. Left as 0 rather than
// invented, same reasoning as LogError below.
func LogParseFailure(model string, failureKind string) {
	LogAutoModeOutcome(OutcomeParseFailure, model, 0, map[string]any{
		"failure_kind": failureKind,
	})
}

func LogError(model string, err error, tooLong bool, durationMs int64) {
	metadata := map[string]any{
		"error": fmt.Sprintf("%v", err),
	}
	if tooLong {
		metadata["transcript_too_long"] = true
	}
	LogAutoModeOutcome(OutcomeError, model, durationMs, metadata)
}
