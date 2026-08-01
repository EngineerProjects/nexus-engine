package rag

import (
	internalrag "github.com/KPO-Tech/seshat/internal/rag"
	publicstorage "github.com/KPO-Tech/seshat/pkg/storage"
	publicvector "github.com/KPO-Tech/seshat/pkg/vector"
)

type (
	Chunk          = internalrag.Chunk
	Chunker        = internalrag.Chunker
	Embedder       = internalrag.Embedder
	IngestRequest  = internalrag.IngestRequest
	IngestResult   = internalrag.IngestResult
	Reranker       = internalrag.Reranker
	SearchRequest  = internalrag.SearchRequest
	SearchResponse = internalrag.SearchResponse
	SearchResult   = internalrag.SearchResult
	Service        = internalrag.Service
	VectorStore    = publicvector.Store

	// ParagraphChunker splits on blank lines with a hard character cap per
	// chunk. The default Chunker (see DefaultChunker) when none is given.
	ParagraphChunker = internalrag.ParagraphChunker

	// SemanticChunker groups sentences into chunks by embedding-similarity
	// instead of blind paragraph splitting - meaningfully better retrieval
	// quality, at the cost of one extra embedding call per sentence during
	// ingest. Falls back to ParagraphChunker behavior if the embedder errors.
	SemanticChunker = internalrag.SemanticChunker
)

func NewService(artifacts publicstorage.ArtifactStore, vectors publicvector.Store, embedder Embedder, chunker Chunker) *Service {
	return internalrag.NewService(artifacts, vectors, embedder, chunker)
}

// DefaultChunker returns a ParagraphChunker with sensible defaults - what
// NewService uses internally when chunker is nil.
func DefaultChunker() Chunker {
	return internalrag.DefaultChunker()
}

// NewSemanticChunker creates a SemanticChunker. threshold <= 0 uses the
// default (0.3) - see SemanticChunker's doc for the tradeoff it makes.
func NewSemanticChunker(embedder Embedder, threshold float32) *SemanticChunker {
	return internalrag.NewSemanticChunker(embedder, threshold)
}

// ArtifactKey builds the deterministic artifact key for a file within a
// corpus (format: "rag/{corpusID}/{fileID}"), matching what Ingest uses
// internally when IngestRequest.FileID is set. Useful for a caller building
// their own delete-by-file tooling on top of Service.DeleteFileChunks.
func ArtifactKey(corpusID, fileID string) string {
	return internalrag.ArtifactKey(corpusID, fileID)
}
