// Package textquality detects text that was technically extracted from a
// document but is not actually usable - the classic "broken font encoding"
// failure mode of naive PDF/document text extraction, distinct from "there
// isn't much text" (see pdftext.Result.Sparse for that check). A PDF using
// a subsetted font with a missing or malformed ToUnicode character map can
// still yield plenty of characters per page, just not real ones.
package textquality

import (
	"regexp"
	"strings"
	"unicode"
)

// cidPlaceholder matches "(cid:123)"-style placeholders. PDF text
// extractors emit these literally when a font's internal glyph ID has no
// mapping back to a real Unicode code point - seeing even one is an
// unambiguous sign of broken extraction, not just messy text.
var cidPlaceholder = regexp.MustCompile(`\(cid:\d+\)`)

// privateUseRatioThreshold: real text should contain ~0% Private Use Area
// code points. A subsetted font's glyph IDs getting misinterpreted as
// Unicode code points characteristically lands many of them in the PUA
// (U+E000-U+F8FF and the two supplementary PUA planes) - anything above a
// small ratio is a strong signal, not noise.
const privateUseRatioThreshold = 0.05

// IsGarbledText reports whether s looks like broken text extraction rather
// than genuine content. Two independent signals, either sufficient on its
// own:
//   - literal "(cid:NNN)" placeholders anywhere in s
//   - more than 5% of runes falling in a Unicode Private Use Area
//
// An empty or whitespace-only string is not "garbled" - that's a separate,
// simpler case (see pdftext.Result.Sparse) callers should check first.
func IsGarbledText(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	if cidPlaceholder.MatchString(s) {
		return true
	}
	total := 0
	privateUse := 0
	for _, r := range s {
		total++
		if unicode.Is(unicode.Co, r) {
			privateUse++
		}
	}
	if total == 0 {
		return false
	}
	return float64(privateUse)/float64(total) > privateUseRatioThreshold
}
