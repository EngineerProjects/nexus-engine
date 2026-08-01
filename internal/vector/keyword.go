package vector

import "strings"

// keywordScore is a lightweight lexical relevance score used where no real
// BM25/full-text index is available: HNSW's hybrid blend, and vectorless
// (no-embedder) search on backends without FTS. It has no IDF term and no
// document-length normalization, so it is not real BM25 - but it scores
// repeated term hits higher than a single hit, matching BM25's core
// term-frequency intuition instead of just checking presence/absence.
// SQLite and pgvector should always be preferred for vectorless search
// where available - they run real BM25/ts_rank through the database.
func keywordScore(text, queryText string) float32 {
	tokens := strings.Fields(strings.ToLower(queryText))
	if len(tokens) == 0 {
		return 0
	}
	lower := strings.ToLower(text)
	var hits float32
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		hits += float32(strings.Count(lower, tok))
	}
	return hits / float32(len(tokens))
}
