// Package officetext re-exports internal/officetext's native DOCX/PPTX/XLSX
// text extraction for external consumers (e.g. seshat-ai's upload and RAG
// ingestion pipelines) that want the same conversion the read_file tool uses
// without going through docling-serve.
package officetext

import internalofficetext "github.com/KPO-Tech/seshat/internal/officetext"

// SupportedExtensions lists the formats Extract can handle natively.
var SupportedExtensions = internalofficetext.SupportedExtensions

// ErrEmpty is returned when a document parses but yields no extractable text.
var ErrEmpty = internalofficetext.ErrEmpty

// Extract converts a DOCX/PPTX/XLSX file's bytes to markdown. ok is false
// when filename's extension isn't natively supported.
func Extract(filename string, data []byte) (markdown string, ok bool, err error) {
	return internalofficetext.Extract(filename, data)
}
