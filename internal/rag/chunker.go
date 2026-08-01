package rag

import (
	"context"
	"strings"
)

// ParagraphChunker splits on blank lines and hard-caps each paragraph at
// MaxChunkChars characters. It does not call any remote service.
type ParagraphChunker struct {
	MaxChunkChars int
}

// DefaultChunker returns a ParagraphChunker with sensible defaults.
func DefaultChunker() Chunker {
	return ParagraphChunker{MaxChunkChars: 800}
}

func (c ParagraphChunker) Split(_ context.Context, text string) ([]Chunk, error) {
	maxChars := c.MaxChunkChars
	if maxChars <= 0 {
		maxChars = 800
	}
	parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	chunks := make([]Chunk, 0, len(parts))
	position := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Slice by rune, not byte: a byte-index cut can land inside a
		// multi-byte UTF-8 character (e.g. accented French text) and
		// corrupt it at the chunk boundary.
		runes := []rune(part)
		for len(runes) > maxChars {
			chunks = append(chunks, Chunk{
				Text:     strings.TrimSpace(string(runes[:maxChars])),
				Position: position,
			})
			position++
			runes = []rune(strings.TrimSpace(string(runes[maxChars:])))
		}
		if len(runes) > 0 {
			chunks = append(chunks, Chunk{
				Text:     string(runes),
				Position: position,
			})
			position++
		}
	}
	return chunks, nil
}
