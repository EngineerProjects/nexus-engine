package reranker

import "os"

// LangSearchReranker is HTTPReranker preconfigured for LangSearch's hosted
// rerank API (langsearch.com - no credit card required for the free tier).
// Kept as a distinct name for backward compatibility; for a fully free,
// self-hosted alternative requiring no API key, use New/NewFromEnv with a
// BaseURL pointing at a local TEI or vLLM instance instead - see
// reranker.go's package doc.
type LangSearchReranker = HTTPReranker

// NewLangSearchReranker reads LANGSEARCH_API_KEY from the environment.
func NewLangSearchReranker() *LangSearchReranker {
	return NewLangSearchRerankerWithKey(os.Getenv("LANGSEARCH_API_KEY"))
}

// NewLangSearchRerankerWithKey builds a LangSearch-configured reranker.
// An empty apiKey produces an unconfigured reranker (IsConfigured() ==
// false) rather than one pointed at LangSearch with no credentials, since
// LangSearch's API requires a key - matches the pre-generalization behavior.
func NewLangSearchRerankerWithKey(apiKey string) *LangSearchReranker {
	cfg := Config{APIKey: apiKey, Model: langSearchModel}
	if apiKey != "" {
		cfg.BaseURL = langSearchBaseURL
	}
	return New(cfg)
}
