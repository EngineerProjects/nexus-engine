package edit

import (
	"strings"
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
	}
	for _, tc := range cases {
		if got := levenshteinDistance([]rune(tc.a), []rune(tc.b)); got != tc.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestLevenshteinSimilarity(t *testing.T) {
	t.Parallel()
	if sim := LevenshteinSimilarity("hello world", "hello world"); sim != 1 {
		t.Errorf("identical strings should be 100%% similar, got %v", sim)
	}
	if sim := LevenshteinSimilarity("hello world", "goodbye moon"); sim > 0.5 {
		t.Errorf("very different strings should score low, got %v", sim)
	}
}

func TestFindFuzzyMatch_WhitespaceDrift(t *testing.T) {
	t.Parallel()
	fileContent := "func main() {\n\tfmt.Println(\"hello\")\n\treturn\n}\n"
	// A model-recalled old_string with a trailing space it shouldn't have -
	// exactly the kind of near-miss FindActualString correctly rejects but
	// which a human/model would immediately recognize as "that one, right there".
	searchString := "\tfmt.Println(\"hello\") \n\treturn"

	match, ok := FindFuzzyMatch(fileContent, searchString)
	if !ok {
		t.Fatal("expected a fuzzy match for a near-identical block")
	}
	if !strings.Contains(match.Text, "fmt.Println(\"hello\")") {
		t.Errorf("expected the matched block to contain the real line, got %q", match.Text)
	}
	if match.Similarity < FuzzyMatchMinSimilarity {
		t.Errorf("match similarity %v below threshold %v", match.Similarity, FuzzyMatchMinSimilarity)
	}
}

func TestFindFuzzyMatch_NoReasonableCandidate(t *testing.T) {
	t.Parallel()
	fileContent := "completely unrelated content\nabout something else entirely\n"
	searchString := "def totally_different_function(x, y, z):\n    return x + y + z"

	if _, ok := FindFuzzyMatch(fileContent, searchString); ok {
		t.Error("expected no fuzzy match for genuinely unrelated content")
	}
}

func TestFindFuzzyMatch_ExactSubstringStillMatches(t *testing.T) {
	t.Parallel()
	fileContent := "line one\nline two\nline three\n"
	match, ok := FindFuzzyMatch(fileContent, "line two")
	if !ok || match.Similarity != 1 {
		t.Errorf("expected a perfect match for an exact substring, got ok=%v sim=%v", ok, match.Similarity)
	}
}

func TestCharDiff_MarksInsertionsAndDeletions(t *testing.T) {
	t.Parallel()
	// "hello world" -> "hello there" : "world" deleted, "there" inserted,
	// shared "hello " kept as common (untagged) text.
	diff := CharDiff("hello world", "hello there")
	if !strings.Contains(diff, "hello ") {
		t.Errorf("expected the common prefix untagged in the diff, got %q", diff)
	}
	if !strings.Contains(diff, "{-") || !strings.Contains(diff, "-}") {
		t.Errorf("expected a deletion marker in the diff, got %q", diff)
	}
	if !strings.Contains(diff, "{+") || !strings.Contains(diff, "+}") {
		t.Errorf("expected an insertion marker in the diff, got %q", diff)
	}
}

func TestCharDiff_IdenticalStringsHaveNoMarkers(t *testing.T) {
	t.Parallel()
	diff := CharDiff("same", "same")
	if strings.Contains(diff, "{-") || strings.Contains(diff, "{+") {
		t.Errorf("expected no diff markers for identical strings, got %q", diff)
	}
}
