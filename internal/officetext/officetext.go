package officetext

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SupportedExtensions lists the formats this package can extract without any
// external service. Kept as the single source of truth for "is this a
// native-office format" checks - see internal/tools/files/read/detector.go.
var SupportedExtensions = map[string]bool{
	".docx": true,
	".pptx": true,
	".xlsx": true,
}

// Extract dispatches to the format-specific extractor based on filename
// extension. ok is false when the extension isn't one this package handles;
// callers should fall back to another conversion path (e.g. docling) in that
// case rather than treating it as an error.
func Extract(filename string, data []byte) (markdown string, ok bool, err error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".docx":
		md, err := ExtractDOCX(data)
		return md, true, emptyAsErr(md, err)
	case ".pptx":
		md, err := ExtractPPTX(data)
		return md, true, emptyAsErr(md, err)
	case ".xlsx":
		md, err := ExtractXLSX(data)
		return md, true, emptyAsErr(md, err)
	default:
		return "", false, nil
	}
}

func emptyAsErr(md string, err error) error {
	if err != nil {
		return err
	}
	if strings.TrimSpace(md) == "" {
		return ErrEmpty
	}
	return nil
}

// ErrEmpty is returned by format extractors when the document parsed
// successfully but yielded no extractable text (e.g. an image-only slide
// deck) - callers can use this to decide whether to still try docling.
var ErrEmpty = fmt.Errorf("no extractable text found")
