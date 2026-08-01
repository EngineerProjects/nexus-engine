package embedder

import "testing"

func TestFromEnv_PublicWrapper(t *testing.T) {
	t.Setenv("RAG_EMBEDDING_URL", "http://localhost:11434")
	t.Setenv("RAG_EMBEDDING_MODEL", "nomic-embed-text")
	t.Setenv("RAG_EMBEDDING_API_KEY", "")
	t.Setenv("RAG_EMBEDDING_PROVIDER", "")

	cfg := FromEnv()
	if cfg.BaseURL != "http://localhost:11434" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Model != "nomic-embed-text" {
		t.Errorf("Model = %q", cfg.Model)
	}
}

func TestNewFromEnv_PublicWrapper_NilWhenUnconfigured(t *testing.T) {
	t.Setenv("RAG_EMBEDDING_URL", "")
	t.Setenv("RAG_EMBEDDING_MODEL", "")

	if e := NewFromEnv(); e != nil {
		t.Errorf("expected nil Embedder when unconfigured, got %v", e)
	}
}

func TestNewFromEnv_PublicWrapper_NonNilWhenConfigured(t *testing.T) {
	t.Setenv("RAG_EMBEDDING_URL", "http://localhost:11434")
	t.Setenv("RAG_EMBEDDING_MODEL", "nomic-embed-text")

	if e := NewFromEnv(); e == nil {
		t.Error("expected a non-nil Embedder when RAG_EMBEDDING_URL/MODEL are set")
	}
}
