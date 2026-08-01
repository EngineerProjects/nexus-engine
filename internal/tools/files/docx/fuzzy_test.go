package docx

import "testing"

func TestStripTags(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<w:t xml:space="preserve">Hello world</w:t>`, "Hello world"},
		{`<w:p><w:r><w:t>a</w:t></w:r></w:p>`, "a"},
		{"no tags here", "no tags here"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := stripTags(tc.in); got != tc.want {
			t.Errorf("stripTags(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFindFuzzyMatch_ScoresOnStrippedText(t *testing.T) {
	pretty := `<w:body>
<w:p>
<w:r>
<w:t xml:space="preserve">The quick brown fox jumps over the lazy dog.</w:t>
</w:r>
</w:p>
</w:body>`

	// Bare text near-miss (no XML tags at all, the natural thing a model
	// would write) - must still score well against the tag-wrapped line.
	match, ok := findFuzzyMatch(pretty, "The quick brown fox jumps over the lazy dog!")
	if !ok {
		t.Fatal("expected a fuzzy match despite the tag-wrapping diluting a naive comparison")
	}
	if match.Similarity < 0.9 {
		t.Errorf("expected a high similarity once tags are stripped from scoring, got %v", match.Similarity)
	}
	// The returned Text is still the ORIGINAL tag-included line - callers
	// need that to build a correct replacement.
	if match.Text == "" || match.Text[0] != '<' {
		t.Errorf("expected FuzzyMatch.Text to be the tag-included original line, got %q", match.Text)
	}
}

func TestFindFuzzyMatch_NoReasonableCandidate(t *testing.T) {
	pretty := `<w:body>
<w:p>
<w:r>
<w:t>completely unrelated content</w:t>
</w:r>
</w:p>
</w:body>`
	if _, ok := findFuzzyMatch(pretty, "something entirely different and much longer than the source"); ok {
		t.Error("expected no fuzzy match for genuinely unrelated text")
	}
}
