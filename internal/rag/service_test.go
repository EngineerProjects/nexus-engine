package rag

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/KPO-Tech/seshat/internal/storage"
	"github.com/KPO-Tech/seshat/internal/vector"
)

// fakeEmbedder returns deterministic embeddings based on keyword content.
type fakeEmbedder struct{}

func (fakeEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "alpha"):
			out = append(out, []float32{1, 0})
		case strings.Contains(lower, "beta"):
			out = append(out, []float32{0, 1})
		default:
			out = append(out, []float32{1, 1})
		}
	}
	return out, nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	tmpDir := t.TempDir()
	storage.SetConfig(storage.Config{
		Provider:  storage.ProviderLocal,
		LocalPath: tmpDir,
	})
	t.Cleanup(storage.ResetProvider)
	artifacts, err := storage.DefaultArtifactStore()
	if err != nil {
		t.Fatalf("DefaultArtifactStore: %v", err)
	}
	return NewService(artifacts, vector.NewMemoryStore(), fakeEmbedder{}, nil)
}

// newVectorlessTestService mirrors newTestService but with no embedder at
// all, matching how buildRAGService constructs a Service when no embedding
// provider is configured (vectorless/BM25-only mode).
func newVectorlessTestService(t *testing.T) *Service {
	t.Helper()
	tmpDir := t.TempDir()
	storage.SetConfig(storage.Config{
		Provider:  storage.ProviderLocal,
		LocalPath: tmpDir,
	})
	t.Cleanup(storage.ResetProvider)
	artifacts, err := storage.DefaultArtifactStore()
	if err != nil {
		t.Fatalf("DefaultArtifactStore: %v", err)
	}
	return NewService(artifacts, vector.NewMemoryStore(), nil, nil)
}

// poisonedEmbedder fails the test if EmbedTexts is ever called - used to
// prove a code path skips embedding entirely rather than merely ignoring
// the result.
type poisonedEmbedder struct{ t *testing.T }

func (p poisonedEmbedder) EmbedTexts(_ context.Context, _ []string) ([][]float32, error) {
	p.t.Helper()
	p.t.Fatal("EmbedTexts should not have been called")
	return nil, nil
}

// stubReranker reverses the input order, so a test can tell whether it ran.
type stubReranker struct{ configured bool }

func (r stubReranker) IsConfigured() bool { return r.configured }

func (r stubReranker) Rerank(_ context.Context, _ string, docs []string, topN int) ([]int, []float32, error) {
	indices := make([]int, len(docs))
	scores := make([]float32, len(docs))
	for i := range docs {
		indices[i] = len(docs) - 1 - i
		scores[i] = float32(len(docs) - i)
	}
	if topN > 0 && topN < len(indices) {
		indices = indices[:topN]
		scores = scores[:topN]
	}
	return indices, scores, nil
}

func TestServiceIngestAndSearch(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	ingested, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb",
		Filename: "notes.txt",
		Text:     "alpha section\n\nbeta section",
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if ingested.Chunks != 2 {
		t.Fatalf("expected 2 chunks, got %d", ingested.Chunks)
	}

	resp, err := svc.Search(ctx, SearchRequest{CorpusID: "kb", Query: "alpha", TopK: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if !strings.Contains(strings.ToLower(resp.Results[0].Text), "alpha") {
		t.Fatalf("unexpected top match: %s", resp.Results[0].Text)
	}
}

func TestServiceSearch_WithFilter(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	_, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "filtered",
		FileID:   "doc-a",
		Filename: "doc-a.txt",
		Text:     "alpha content here",
	})
	if err != nil {
		t.Fatalf("Ingest A: %v", err)
	}
	_, err = svc.Ingest(ctx, IngestRequest{
		CorpusID: "filtered",
		FileID:   "doc-b",
		Filename: "doc-b.txt",
		Text:     "alpha content there",
	})
	if err != nil {
		t.Fatalf("Ingest B: %v", err)
	}

	// Filter to only doc-a by filename.
	resp, err := svc.Search(ctx, SearchRequest{
		CorpusID: "filtered",
		Query:    "alpha",
		TopK:     10,
		Filter:   map[string]any{"filename": "doc-a.txt"},
	})
	if err != nil {
		t.Fatalf("Search with filter: %v", err)
	}
	for _, r := range resp.Results {
		if r.Metadata["filename"] != "doc-a.txt" {
			t.Errorf("filter leaked result from %s", r.Metadata["filename"])
		}
	}
}

func TestServiceIngestAndSearch_VectorlessWithoutEmbedder(t *testing.T) {
	ctx := context.Background()
	svc := newVectorlessTestService(t)

	ingested, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb",
		Filename: "notes.txt",
		Text:     "the quick brown fox\n\nan entirely unrelated paragraph",
	})
	if err != nil {
		t.Fatalf("Ingest without an embedder: %v", err)
	}
	if ingested.Chunks != 2 {
		t.Fatalf("expected 2 chunks, got %d", ingested.Chunks)
	}

	resp, err := svc.Search(ctx, SearchRequest{CorpusID: "kb", Query: "fox", TopK: 5})
	if err != nil {
		t.Fatalf("Search without an embedder: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected exactly 1 BM25 match for 'fox', got %d: %+v", len(resp.Results), resp.Results)
	}
	if !strings.Contains(resp.Results[0].Text, "fox") {
		t.Errorf("unexpected match: %q", resp.Results[0].Text)
	}
}

func TestServiceSearch_HybridWeightOneSkipsEmbeddingEvenWithEmbedder(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	storage.SetConfig(storage.Config{Provider: storage.ProviderLocal, LocalPath: tmpDir})
	t.Cleanup(storage.ResetProvider)
	artifacts, err := storage.DefaultArtifactStore()
	if err != nil {
		t.Fatalf("DefaultArtifactStore: %v", err)
	}
	svc := NewService(artifacts, vector.NewMemoryStore(), poisonedEmbedder{t: t}, nil)

	// Ingest must go through fakeEmbedder-equivalent text-only path too, so
	// use a real (non-poisoned) embedder just for ingest, then swap it out.
	ingestSvc := NewService(artifacts, svc.Vectors(), fakeEmbedder{}, nil)
	if _, err := ingestSvc.Ingest(ctx, IngestRequest{
		CorpusID: "kb", Filename: "doc.txt", Text: "alpha content here",
	}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// hybrid_weight=1 (pure keyword) with an embedder configured must not
	// call EmbedTexts at all - poisonedEmbedder fails the test if it does.
	if _, err := svc.Search(ctx, SearchRequest{
		CorpusID: "kb", Query: "alpha", TopK: 5, HybridWeight: 1,
	}); err != nil {
		t.Fatalf("Search with hybrid_weight=1: %v", err)
	}
}

func TestServiceSearch_VectorlessRerankStillApplies(t *testing.T) {
	ctx := context.Background()
	svc := newVectorlessTestService(t)
	svc.SetReranker(stubReranker{configured: true})

	if _, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb", Filename: "a.txt", Text: "keyword one",
	}); err != nil {
		t.Fatalf("Ingest a: %v", err)
	}
	if _, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb", Filename: "b.txt", Text: "keyword two",
	}); err != nil {
		t.Fatalf("Ingest b: %v", err)
	}

	resp, err := svc.Search(ctx, SearchRequest{CorpusID: "kb", Query: "keyword", TopK: 5})
	if err != nil {
		t.Fatalf("vectorless Search with reranker: %v", err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results from vectorless search + rerank")
	}
}

func TestServiceIngest_SameFileIDReplacesRatherThanDuplicates(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	store := svc.vectors

	if _, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb", FileID: "doc", Filename: "doc.txt",
		Text: "alpha one\n\nalpha two\n\nalpha three",
	}); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	first, err := store.Get(ctx, "kb", nil)
	if err != nil {
		t.Fatalf("Get after first ingest: %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("expected 3 records after first ingest, got %d", len(first))
	}

	// Re-ingest the same file_id with the same chunk count - should replace
	// the 3 existing records in place, not add 3 more.
	if _, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb", FileID: "doc", Filename: "doc.txt",
		Text: "alpha uno\n\nalpha dos\n\nalpha tres",
	}); err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	second, err := store.Get(ctx, "kb", nil)
	if err != nil {
		t.Fatalf("Get after second ingest: %v", err)
	}
	if len(second) != 3 {
		t.Fatalf("expected 3 records after re-ingest (replaced, not duplicated), got %d", len(second))
	}
	for _, r := range second {
		if strings.Contains(r.Text, "one") || strings.Contains(r.Text, "two") || strings.Contains(r.Text, "three") {
			t.Errorf("found stale chunk from first ingest still present: %q", r.Text)
		}
	}
}

func TestServiceIngest_ShrunkFileRemovesStaleTrailingChunks(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	store := svc.vectors

	if _, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb", FileID: "doc", Filename: "doc.txt",
		Text: "alpha one\n\nalpha two\n\nalpha three\n\nalpha four",
	}); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	first, err := store.Get(ctx, "kb", nil)
	if err != nil {
		t.Fatalf("Get after first ingest: %v", err)
	}
	if len(first) != 4 {
		t.Fatalf("expected 4 records after first ingest, got %d", len(first))
	}

	// Re-ingest a much shorter version under the same file_id.
	result, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb", FileID: "doc", Filename: "doc.txt",
		Text: "alpha only",
	})
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if result.Chunks != 1 {
		t.Fatalf("expected 1 chunk in the shorter version, got %d", result.Chunks)
	}

	second, err := store.Get(ctx, "kb", nil)
	if err != nil {
		t.Fatalf("Get after second ingest: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected the 3 stale trailing chunks to be cleaned up, got %d records: %+v", len(second), second)
	}
}

// ─── Chunker tests ────────────────────────────────────────────────────────────

func TestParagraphChunker_Split(t *testing.T) {
	ctx := context.Background()
	c := ParagraphChunker{MaxChunkChars: 50}
	chunks, err := c.Split(ctx, "First paragraph.\n\nSecond paragraph.\n\nThird.")
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Position != 0 || chunks[1].Position != 1 {
		t.Fatalf("positions wrong: %v", chunks)
	}
}

func TestParagraphChunker_HardCap(t *testing.T) {
	ctx := context.Background()
	c := ParagraphChunker{MaxChunkChars: 10}
	chunks, err := c.Split(ctx, "abcdefghijklmnopqrstuvwxyz") // 26 chars, no newlines
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for text exceeding MaxChunkChars, got %d", len(chunks))
	}
	for _, ch := range chunks {
		if len(ch.Text) > 10 {
			t.Errorf("chunk exceeds cap: %q (%d chars)", ch.Text, len(ch.Text))
		}
	}
}

func TestParagraphChunker_Split_RuneSafeWithMultibyteText(t *testing.T) {
	ctx := context.Background()
	// "é", "è", "à" are 2-byte UTF-8 sequences - a byte-index cut has good
	// odds of landing inside one of them for a maxChars small enough to
	// force a split partway through this sentence.
	text := strings.Repeat("café à côté déjà générée ", 20)
	c := ParagraphChunker{MaxChunkChars: 37}
	chunks, err := c.Split(ctx, text)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks to exercise the cut boundary, got %d", len(chunks))
	}
	for i, ch := range chunks {
		if !utf8.ValidString(ch.Text) {
			t.Errorf("chunk %d is not valid UTF-8: %q", i, ch.Text)
		}
		if strings.ContainsRune(ch.Text, utf8.RuneError) {
			t.Errorf("chunk %d contains a replacement rune (corrupted multi-byte char): %q", i, ch.Text)
		}
	}
	// Reassembling should reproduce the same runes with no loss (modulo the
	// whitespace TrimSpace already normalizes at each boundary).
	var rebuilt strings.Builder
	for _, ch := range chunks {
		rebuilt.WriteString(ch.Text)
		rebuilt.WriteByte(' ')
	}
	if !strings.Contains(rebuilt.String(), "café") || !strings.Contains(rebuilt.String(), "générée") {
		t.Errorf("expected accented words to survive intact across chunk boundaries, got: %q", rebuilt.String())
	}
}

func TestSemanticChunker_FallbackOnEmbedderError(t *testing.T) {
	ctx := context.Background()
	// An embedder that always fails.
	errEmbed := &errorEmbedder{}
	sc := NewSemanticChunker(errEmbed, 0.3)
	// Should fall back to ParagraphChunker without returning an error.
	chunks, err := sc.Split(ctx, "First paragraph.\n\nSecond paragraph.")
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk on fallback")
	}
}

func TestSemanticChunker_GroupsBySimilarity(t *testing.T) {
	ctx := context.Background()
	// Embedder that returns identical vectors for "A" sentences and orthogonal
	// vectors for "B" sentences, so the chunker splits at the boundary.
	sc := NewSemanticChunker(semanticTestEmbedder{}, 0.5)
	sc.MinChunkChars = 1
	text := "A sentence one. A sentence two. B sentence three. B sentence four."
	chunks, err := sc.Split(ctx, text)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// At least two chunks: the A group and the B group.
	if len(chunks) < 2 {
		t.Fatalf("expected ≥2 semantic chunks, got %d: %v", len(chunks), chunks)
	}
}

func TestSplitSentences_Basic(t *testing.T) {
	sentences := splitSentences("Hello world. How are you? Fine!")
	if len(sentences) < 2 {
		t.Fatalf("expected at least 2 sentences, got %d: %v", len(sentences), sentences)
	}
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

type errorEmbedder struct{}

func (errorEmbedder) EmbedTexts(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("embedding service down")
}

// semanticTestEmbedder returns [1,0] for sentences containing "A" and [0,1] for "B".
type semanticTestEmbedder struct{}

func (semanticTestEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if strings.Contains(t, "A") {
			out[i] = []float32{1, 0}
		} else {
			out[i] = []float32{0, 1}
		}
	}
	return out, nil
}
