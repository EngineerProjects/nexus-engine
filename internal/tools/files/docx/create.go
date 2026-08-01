package docx

import (
	"archive/zip"
	"bytes"
	"regexp"
	"strings"
)

var headingLinePattern = regexp.MustCompile(`^(#{1,6})\s+(.+)`)

// escapeXML escapes the five characters that are ever meaningful inside a
// DOCX text node or attribute value.
func escapeXML(text string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(text)
}

// buildParagraphsXML converts plain text into WordprocessingML paragraphs:
// one paragraph per line, "#".."######" prefixes become Heading1..6 styled
// paragraphs (same convention write_file uses for markdown-ish structure),
// blank lines become empty paragraphs.
func buildParagraphsXML(content string) string {
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if m := headingLinePattern.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			b.WriteString(`<w:p><w:pPr><w:pStyle w:val="Heading`)
			b.WriteString(itoa(level))
			b.WriteString(`"/></w:pPr><w:r><w:t>`)
			b.WriteString(escapeXML(m[2]))
			b.WriteString(`</w:t></w:r></w:p>`)
			continue
		}
		if strings.TrimSpace(line) == "" {
			b.WriteString(`<w:p/>`)
			continue
		}
		b.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
		b.WriteString(escapeXML(line))
		b.WriteString(`</w:t></w:r></w:p>`)
	}
	return b.String()
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	digits := "0123456789"
	var out []byte
	for n > 0 {
		out = append([]byte{digits[n%10]}, out...)
		n /= 10
	}
	return string(out)
}

// createMinimalDocx builds a complete, valid, minimal DOCX file from plain
// text - just enough structure ([Content_Types].xml, root/document rels,
// a Normal + Heading1-3 style sheet, and the body itself) for Word/LibreOffice
// and this package's own readDocumentXML to open it. Mirrors
// DesktopCommanderMCP's docx.ts createMinimalDocxZip + its write() paragraph
// construction.
func createMinimalDocx(content string) ([]byte, error) {
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + buildParagraphsXML(content) +
		`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>` +
		`</w:body></w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
			`</Relationships>`,
		"word/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:docDefaults><w:rPrDefault><w:rPr>` +
			`<w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>` +
			`<w:sz w:val="22"/><w:szCs w:val="22"/>` +
			`</w:rPr></w:rPrDefault></w:docDefaults>` +
			`<w:style w:type="paragraph" w:styleId="Normal" w:default="1"><w:name w:val="Normal"/></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="32"/></w:rPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:pPr><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:b/><w:sz w:val="28"/></w:rPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:pPr><w:outlineLvl w:val="2"/></w:pPr><w:rPr><w:b/><w:sz w:val="24"/></w:rPr></w:style>` +
			`</w:styles>`,
		"word/document.xml": documentXML,
	}

	// Deterministic order for reproducible output (map iteration order isn't).
	order := []string{
		"[Content_Types].xml", "_rels/.rels", "word/_rels/document.xml.rels",
		"word/styles.xml", "word/document.xml",
	}
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
