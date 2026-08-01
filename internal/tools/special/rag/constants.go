package ragtool

const (
	ToolSearchName = "rag_search"
	ToolIngestName = "rag_ingest"
	ToolDeleteName = "rag_delete"

	SearchHint = "Search a document corpus using semantic similarity. Use when documents have been ingested and you need to find relevant passages."
	IngestHint = "Ingest a text document into a named corpus for later semantic search. Returns the number of indexed chunks."
	DeleteHint = "Delete an entire corpus, or a single file's chunks within a corpus."

	DefaultTopK = 5

	// deleteFileChunkCeiling bounds how many chunk-position keys are
	// speculatively deleted when removing a single file's chunks by
	// file_id/filename - see staleChunkCleanupCeiling in internal/rag for
	// why a generous blind range is safe and cheap.
	deleteFileChunkCeiling = 2000
)
