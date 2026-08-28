package browser

import "testing"

// TestApplyScreenshotByteCap guards against regressing screenshot payloads
// back to being unconditionally inlined - unlike Snapshot's Text (capped at
// MaxSnapshotText), nothing previously bounded screenshot size, so a single
// large capture (or several retained across a long tool-use-heavy turn)
// could inflate conversation token usage unboundedly.
func TestApplyScreenshotByteCap(t *testing.T) {
	t.Run("inlines data under the cap", func(t *testing.T) {
		image := make([]byte, 100)
		result := &Screenshot{}
		applyScreenshotByteCap(result, image, 200)
		if result.Truncated {
			t.Fatal("expected Truncated to be false when under the cap")
		}
		if result.DataBase64 == "" {
			t.Fatal("expected DataBase64 to be populated when under the cap")
		}
	})

	t.Run("truncates data over the cap", func(t *testing.T) {
		image := make([]byte, 300)
		result := &Screenshot{}
		applyScreenshotByteCap(result, image, 200)
		if !result.Truncated {
			t.Fatal("expected Truncated to be true when over the cap")
		}
		if result.DataBase64 != "" {
			t.Fatal("expected DataBase64 to be empty when over the cap")
		}
	})

	t.Run("cap disabled when maxBytes is zero or negative", func(t *testing.T) {
		image := make([]byte, 1_000_000)
		result := &Screenshot{}
		applyScreenshotByteCap(result, image, 0)
		if result.Truncated {
			t.Fatal("expected no cap to be applied when maxBytes <= 0")
		}
		if result.DataBase64 == "" {
			t.Fatal("expected DataBase64 to be populated when the cap is disabled")
		}
	})
}
