package textquality

import (
	"strings"
	"testing"
)

func TestIsGarbledText_RealTextIsNotGarbled(t *testing.T) {
	if IsGarbledText("Bienvenue dans la lecon portant sur le diagnostic financier fonctionnel.") {
		t.Fatal("expected real prose to not be flagged as garbled")
	}
}

func TestIsGarbledText_EmptyIsNotGarbled(t *testing.T) {
	if IsGarbledText("") || IsGarbledText("   \n\t") {
		t.Fatal("expected empty/whitespace-only text to not be flagged as garbled - that's Sparse's job, not this check's")
	}
}

func TestIsGarbledText_CIDPlaceholdersAreGarbled(t *testing.T) {
	if !IsGarbledText("Report Title (cid:47)(cid:12) Summary") {
		t.Fatal("expected (cid:NNN) placeholders to be flagged as garbled")
	}
}

func TestIsGarbledText_HighPrivateUseRatioIsGarbled(t *testing.T) {
	// A subsetted font's glyph IDs misread as Unicode - simulate with PUA
	// code points (U+E000 onward) making up the bulk of the string.
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteRune(rune(0xE000 + i))
	}
	if !IsGarbledText(sb.String()) {
		t.Fatal("expected a string dominated by Private Use Area code points to be flagged as garbled")
	}
}

func TestIsGarbledText_ASingleStrayPrivateUseCharIsNotGarbled(t *testing.T) {
	// A stray PUA character (e.g. a custom bullet glyph) shouldn't sink an
	// otherwise normal, long passage of real text.
	var sb strings.Builder
	sb.WriteString(strings.Repeat("This is a perfectly normal sentence with real words. ", 20))
	sb.WriteRune(rune(0xE000))
	if IsGarbledText(sb.String()) {
		t.Fatal("expected a single stray PUA character in a long real passage to stay under the threshold")
	}
}
