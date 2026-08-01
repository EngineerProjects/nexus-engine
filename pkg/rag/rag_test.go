package rag

import (
	"context"
	"strings"
	"testing"

	publicvector "github.com/KPO-Tech/seshat/pkg/vector"
)

// fakeEmbedder is a minimal Embedder for exercising the public API surface
// end-to-end without a real embedding provider.
type fakeEmbedder struct{}

func (fakeEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if strings.Contains(strings.ToLower(t), "alpha") {
			out[i] = []float32{1, 0}
		} else {
			out[i] = []float32{0, 1}
		}
	}
	return out, nil
}

func TestDefaultChunker_PublicWrapper(t *testing.T) {
	chunks, err := DefaultChunker().Split(context.Background(), "First paragraph.\n\nSecond paragraph.")
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestNewSemanticChunker_PublicWrapper(t *testing.T) {
	sc := NewSemanticChunker(fakeEmbedder{}, 0.5)
	chunks, err := sc.Split(context.Background(), "Alpha one. Alpha two.")
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
}

func TestArtifactKey_PublicWrapper(t *testing.T) {
	got := ArtifactKey("corpus", "file-1")
	want := "rag/corpus/file-1"
	if got != want {
		t.Errorf("ArtifactKey = %q, want %q", got, want)
	}
}

// TestService_PublicAPIEndToEnd proves the whole public surface (NewService,
// DefaultChunker, IngestRequest/SearchRequest, and the Service's own methods
// via the Service type alias) works together, not just that it compiles.
func TestService_PublicAPIEndToEnd(t *testing.T) {
	ctx := context.Background()
	svc := NewService(nil, publicvector.NewMemoryStore(), fakeEmbedder{}, DefaultChunker())

	result, err := svc.Ingest(ctx, IngestRequest{
		CorpusID: "kb",
		FileID:   "doc-1",
		Filename: "doc.txt",
		Text:     "alpha section\n\nbeta section",
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if result.Chunks != 2 {
		t.Fatalf("expected 2 chunks, got %d", result.Chunks)
	}
	if result.Artifact.Key != ArtifactKey("kb", "doc-1") {
		t.Errorf("expected deterministic artifact key, got %q", result.Artifact.Key)
	}

	resp, err := svc.Search(ctx, SearchRequest{CorpusID: "kb", Query: "alpha", TopK: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(resp.Results) != 1 || !strings.Contains(resp.Results[0].Text, "alpha") {
		t.Fatalf("unexpected search results: %+v", resp.Results)
	}

	// Service methods reachable through the type alias without any wrapper.
	if err := svc.DeleteNamespace(ctx, "kb"); err != nil {
		t.Fatalf("DeleteNamespace: %v", err)
	}
}
