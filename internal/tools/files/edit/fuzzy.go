package edit

import "strings"

// FuzzyMatchMinSimilarity is the similarity floor below which a fuzzy match
// isn't worth suggesting - a low-similarity "closest" match is more likely
// to mislead than help.
const FuzzyMatchMinSimilarity = 0.7

// FuzzyMatchMaxSearchLines / FuzzyMatchMaxSearchStringLen bound the cost of
// fuzzy search: it's O(totalLines * len(searchString)^2) in the worst case
// (a Levenshtein DP table per candidate window), which is fine for a normal
// edit_file call on a normal file but not something to run unbounded on a
// huge file or a huge old_string.
const (
	FuzzyMatchMaxSearchLines     = 20000
	FuzzyMatchMaxSearchStringLen = 4000
)

// FuzzyMatch is the best candidate found for a failed exact/quote-normalized
// match, with the info needed to build an actionable error message.
type FuzzyMatch struct {
	Text       string
	Similarity float64
}

// FindFuzzyMatch searches fileContent for the substring most similar to
// searchString, using a sliding window sized to searchString's line count
// (edit blocks are almost always a handful of contiguous lines, so this is
// both cheap and matches how the tool is actually used - unlike a full
// rune-by-rune slide, which would be far more expensive for no benefit here).
// Returns ok=false when no candidate clears FuzzyMatchMinSimilarity, or when
// the search space exceeds the bounds above (silently skipped, not an error -
// the caller just won't get a suggestion).
func FindFuzzyMatch(fileContent, searchString string) (FuzzyMatch, bool) {
	searchString = strings.TrimRight(searchString, "\n")
	if searchString == "" || len(searchString) > FuzzyMatchMaxSearchStringLen {
		return FuzzyMatch{}, false
	}

	fileLines := strings.Split(fileContent, "\n")
	if len(fileLines) > FuzzyMatchMaxSearchLines {
		return FuzzyMatch{}, false
	}

	searchLines := strings.Split(searchString, "\n")
	windowSize := len(searchLines)
	if windowSize > len(fileLines) {
		windowSize = len(fileLines)
	}
	if windowSize == 0 {
		return FuzzyMatch{}, false
	}

	best := FuzzyMatch{}
	for start := 0; start+windowSize <= len(fileLines); start++ {
		candidate := strings.Join(fileLines[start:start+windowSize], "\n")
		sim := LevenshteinSimilarity(searchString, candidate)
		if sim > best.Similarity {
			best = FuzzyMatch{Text: candidate, Similarity: sim}
		}
	}

	if best.Similarity < FuzzyMatchMinSimilarity {
		return FuzzyMatch{}, false
	}
	return best, true
}

// LevenshteinSimilarity returns a 0..1 similarity score (1 = identical)
// derived from Levenshtein edit distance, normalized by the longer string's
// length so unequal-length comparisons stay meaningful.
func LevenshteinSimilarity(a, b string) float64 {
	ar, br := []rune(a), []rune(b)
	maxLen := len(ar)
	if len(br) > maxLen {
		maxLen = len(br)
	}
	if maxLen == 0 {
		return 1
	}
	dist := levenshteinDistance(ar, br)
	return 1 - float64(dist)/float64(maxLen)
}

// levenshteinDistance is the classic O(n*m) dynamic-programming edit
// distance, using only two rolling rows (O(min(n,m)) memory).
func levenshteinDistance(a, b []rune) int {
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// CharDiff renders a's and b's difference as DesktopCommanderMCP-style
// inline markup: "common{-removed-}{+added+}common" - built by backtracking
// through the same Levenshtein DP table used for the similarity score, so
// the diff shown is guaranteed to be a minimal edit sequence, not a
// heuristic approximation.
func CharDiff(a, b string) string {
	ar, br := []rune(a), []rune(b)
	n, m := len(ar), len(br)

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := dp[i-1][j] + 1
			ins := dp[i][j-1] + 1
			sub := dp[i-1][j-1] + cost
			v := del
			if ins < v {
				v = ins
			}
			if sub < v {
				v = sub
			}
			dp[i][j] = v
		}
	}

	// Backtrack from (n, m) to (0, 0), collecting ops in reverse.
	type op struct {
		kind byte // 'c' common, 'd' delete (from a), 'i' insert (from b)
		r    rune
	}
	var ops []op
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && ar[i-1] == br[j-1] && dp[i][j] == dp[i-1][j-1]:
			ops = append(ops, op{'c', ar[i-1]})
			i--
			j--
		case i > 0 && j > 0 && dp[i][j] == dp[i-1][j-1]+1:
			ops = append(ops, op{'i', br[j-1]}, op{'d', ar[i-1]})
			i--
			j--
		case i > 0 && dp[i][j] == dp[i-1][j]+1:
			ops = append(ops, op{'d', ar[i-1]})
			i--
		default:
			ops = append(ops, op{'i', br[j-1]})
			j--
		}
	}
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}

	var sb strings.Builder
	flush := func(kind byte, buf *strings.Builder) {
		if buf.Len() == 0 {
			return
		}
		switch kind {
		case 'd':
			sb.WriteString("{-")
			sb.WriteString(buf.String())
			sb.WriteString("-}")
		case 'i':
			sb.WriteString("{+")
			sb.WriteString(buf.String())
			sb.WriteString("+}")
		default:
			sb.WriteString(buf.String())
		}
		buf.Reset()
	}

	var run strings.Builder
	var runKind byte
	for _, o := range ops {
		if o.kind != runKind {
			flush(runKind, &run)
			runKind = o.kind
		}
		run.WriteRune(o.r)
	}
	flush(runKind, &run)

	return sb.String()
}
