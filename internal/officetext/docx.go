package officetext

import (
	"archive/zip"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExtractDOCX converts a Word document's body (paragraphs, headings, tables)
// to markdown. Embedded images, headers/footers and tracked-changes markup
// are not rendered - callers that need those still have docling available.
func ExtractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a valid DOCX (zip open failed): %w", err)
	}

	f := findZipFile(zr, "word/document.xml")
	if f == nil {
		return "", fmt.Errorf("not a valid DOCX: missing word/document.xml")
	}
	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open word/document.xml: %w", err)
	}
	defer rc.Close()

	tree, err := parseXMLTree(rc)
	if err != nil {
		return "", fmt.Errorf("failed to parse word/document.xml: %w", err)
	}

	body := findFirst(tree, "body")
	if body == nil {
		return "", fmt.Errorf("not a valid DOCX: missing w:body")
	}

	var sb strings.Builder
	walkWordBlocks(body, &sb)
	return strings.TrimSpace(sb.String()), nil
}

var headingLevelRe = regexp.MustCompile(`(?i)^heading\s*(\d)$|^heading(\d)$`)

// walkWordBlocks writes markdown for each block-level child of a WordprocessingML
// container (w:body, or a w:sdt content wrapper used by many templates for
// content controls). Anything else (sectPr, bookmarks, ...) is skipped.
func walkWordBlocks(container *node, sb *strings.Builder) {
	for _, c := range container.Children {
		switch c.Local {
		case "p":
			writeWordParagraph(c, sb)
		case "tbl":
			writeMarkdownTable(c, sb)
		case "sdt":
			if content := c.child("sdtContent"); content != nil {
				walkWordBlocks(content, sb)
			}
		}
	}
}

func writeWordParagraph(p *node, sb *strings.Builder) {
	text := strings.TrimSpace(p.text())
	if text == "" {
		return
	}

	level := 0
	if pPr := p.child("pPr"); pPr != nil {
		if style := pPr.child("pStyle"); style != nil {
			level = headingLevel(style.attr("val"))
		}
	}

	if level > 0 {
		if level > 6 {
			level = 6
		}
		sb.WriteString(strings.Repeat("#", level))
		sb.WriteByte(' ')
		sb.WriteString(text)
		sb.WriteString("\n\n")
		return
	}

	sb.WriteString(text)
	sb.WriteString("\n\n")
}

// headingLevel maps a Word paragraph style id (e.g. "Heading1", "Heading 1",
// "Title") to a markdown heading level, or 0 for a body-text style.
func headingLevel(styleID string) int {
	if strings.EqualFold(styleID, "Title") {
		return 1
	}
	m := headingLevelRe.FindStringSubmatch(styleID)
	if m == nil {
		return 0
	}
	digit := m[1]
	if digit == "" {
		digit = m[2]
	}
	n, err := strconv.Atoi(digit)
	if err != nil {
		return 0
	}
	return n
}

// writeMarkdownTable renders a w:tbl (WordprocessingML) or a:tbl (DrawingML,
// used inside PPTX slides) as a GitHub-flavored markdown table. Both dialects
// use the same local element names (tbl/tr/tc), so this is shared by DOCX and
// PPTX extraction.
func writeMarkdownTable(tbl *node, sb *strings.Builder) {
	rows := findAll(tbl, "tr")
	if len(rows) == 0 {
		return
	}

	cellRows := make([][]string, 0, len(rows))
	maxCols := 0
	for _, tr := range rows {
		cells := findAll(tr, "tc")
		row := make([]string, 0, len(cells))
		for _, tc := range cells {
			cellText := strings.Join(strings.Fields(tc.text()), " ")
			cellText = strings.ReplaceAll(cellText, "|", "\\|")
			row = append(row, cellText)
		}
		if len(row) > maxCols {
			maxCols = len(row)
		}
		cellRows = append(cellRows, row)
	}
	if maxCols == 0 {
		return
	}

	writeRow := func(row []string) {
		sb.WriteByte('|')
		for i := 0; i < maxCols; i++ {
			sb.WriteByte(' ')
			if i < len(row) {
				sb.WriteString(row[i])
			}
			sb.WriteString(" |")
		}
		sb.WriteByte('\n')
	}

	writeRow(cellRows[0])
	sb.WriteByte('|')
	for i := 0; i < maxCols; i++ {
		sb.WriteString(" --- |")
	}
	sb.WriteByte('\n')
	for _, row := range cellRows[1:] {
		writeRow(row)
	}
	sb.WriteByte('\n')
}

func findZipFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}
