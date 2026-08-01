package reranker

import internalreranker "github.com/KPO-Tech/seshat/internal/rag/reranker"

type (
	// HTTPReranker speaks the Cohere-compatible rerank API shape, also
	// understood by self-hosted alternatives (HuggingFace TEI, vLLM).
	// LangSearchReranker is HTTPReranker preconfigured for LangSearch's
	// hosted API - both names refer to the same underlying type.
	HTTPReranker       = internalreranker.HTTPReranker
	LangSearchReranker = internalreranker.LangSearchReranker

	// Config configures an HTTPReranker: BaseURL, optional APIKey, and Model.
	Config = internalreranker.Config
)

// New creates an HTTPReranker from an explicit Config - use this to point at
// a self-hosted rerank server (e.g. a local TEI or vLLM instance serving
// BAAI/bge-reranker-v2-m3) instead of LangSearch's hosted API.
func New(cfg Config) *HTTPReranker {
	return internalreranker.New(cfg)
}

// NewFromEnv builds a reranker from RAG_RERANK_URL / RAG_RERANK_MODEL /
// RAG_RERANK_API_KEY, falling back to LangSearch's hosted API when only
// LANGSEARCH_API_KEY is set. See internal/rag/reranker's package doc.
func NewFromEnv() *HTTPReranker {
	return internalreranker.NewFromEnv()
}

// NewLangSearchRerankerWithKey builds a reranker pointed at LangSearch's
// hosted API specifically. An empty apiKey produces an unconfigured
// reranker rather than one pointed at LangSearch with no credentials.
func NewLangSearchRerankerWithKey(apiKey string) *LangSearchReranker {
	return internalreranker.NewLangSearchRerankerWithKey(apiKey)
}
