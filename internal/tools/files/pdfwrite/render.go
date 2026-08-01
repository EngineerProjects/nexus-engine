// Package pdfwrite provides the write_pdf tool: creating a new PDF from
// plain/markdown-ish text, appending text as new page(s) to an existing PDF,
// and deleting pages from an existing PDF. Text layout (word-wrap, automatic
// pagination) is handled by go-pdf/fpdf, a pure-Go library that ships PDF's
// standard 14 fonts (Helvetica, Times, Courier) built in - no font file needs
// to be embedded in this binary, unlike TTF-based PDF libraries. Page-level
// operations (append, delete) reuse pdfcpu, already a seshat dependency
// (internal/tools/files/read/pdf.go uses it for page count/extraction).
package pdfwrite

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/go-pdf/fpdf"
)

// headingLinePattern matches Markdown ATX headings ("#" through "######"),
// the same convention docx_edit uses for its plain-text-to-heading mapping.
var headingLinePattern = regexp.MustCompile(`^(#{1,6})\s+(.+)`)

const (
	pageMargin  = 20.0 // mm
	bodyFontPt  = 11.0
	bodyLineMM  = 6.0
	blankLineMM = 4.0
)

// headingSize returns the font point size for a heading level 1-6.
func headingSize(level int) float64 {
	switch level {
	case 1:
		return 20
	case 2:
		return 17
	case 3:
		return 15
	case 4:
		return 13
	case 5:
		return 12
	default:
		return 11
	}
}

// renderTextToPDF renders plain/markdown-ish content into a complete PDF
// document, word-wrapping paragraphs and paginating automatically as content
// overflows a page. Lines starting with "#".."######" become headings
// (mirroring docx_edit's convention); blank lines become paragraph breaks.
func renderTextToPDF(content string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	pdf.SetAutoPageBreak(true, pageMargin)
	pdf.AddPage()

	// cp1252 (fpdf's standard-font encoding) covers Latin-1 supplement,
	// which handles French accents; unsupported runes are dropped rather
	// than corrupting the PDF.
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetFont("Helvetica", "", bodyFontPt)
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			pdf.Ln(blankLineMM)
			continue
		}
		if m := headingLinePattern.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			size := headingSize(level)
			pdf.SetFont("Helvetica", "B", size)
			pdf.MultiCell(0, size*0.5, tr(strings.TrimSpace(m[2])), "", "L", false)
			pdf.SetFont("Helvetica", "", bodyFontPt)
			pdf.Ln(2)
			continue
		}
		pdf.MultiCell(0, bodyLineMM, tr(line), "", "L", false)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
