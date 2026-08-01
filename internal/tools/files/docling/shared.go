// Package docling groups the explicit, agent-invocable tools built on
// docling-serve: read_document_url (fetch+convert a remote document) and
// docling_convert (convert a local file that native extraction can't
// handle well - scanned PDFs, complex slide decks, audio). Both are opt-in:
// the agent reaches for them deliberately, as opposed to internal/tools/files/read's
// own docling fallback, which stays in place so a scanned PDF or audio file
// still "just works" through plain FileRead without the agent needing to
// know docling exists.
package docling

// Config holds the shared configuration for every tool in this package.
type Config struct {
	// DoclingURL is the base URL of a running docling-serve instance.
	// When empty, tools register but always report "not configured".
	DoclingURL string
}
