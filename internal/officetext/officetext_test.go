package officetext

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Real-world fixtures ---------------------------------------------------
// testdata/sample.docx and testdata/sample.xlsx were produced by a real
// LibreOffice headless conversion (see testdata/sample.md, testdata/sample.csv
// for the sources), not hand-crafted - they exercise the actual OOXML a real
// office suite emits, not just what this package's own writer would produce.

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read testdata/%s: %v", name, err)
	}
	return data
}

func TestExtractDOCX_RealFile(t *testing.T) {
	data := readTestdata(t, "sample.docx")
	md, err := ExtractDOCX(data)
	if err != nil {
		t.Fatalf("ExtractDOCX: %v", err)
	}

	if !strings.Contains(md, "# Sample Report") {
		t.Errorf("expected top-level heading in output, got:\n%s", md)
	}
	if !strings.Contains(md, "## Section One") {
		t.Errorf("expected level-2 heading in output, got:\n%s", md)
	}
	if !strings.Contains(md, "plain") {
		t.Errorf("expected paragraph text in output, got:\n%s", md)
	}
	if !strings.Contains(md, "Alice") || !strings.Contains(md, "90") {
		t.Errorf("expected table cell content in output, got:\n%s", md)
	}
	if !strings.Contains(md, "|") {
		t.Errorf("expected a markdown table in output, got:\n%s", md)
	}
}

func TestExtractXLSX_RealFile(t *testing.T) {
	data := readTestdata(t, "sample.xlsx")
	md, err := ExtractXLSX(data)
	if err != nil {
		t.Fatalf("ExtractXLSX: %v", err)
	}

	for _, want := range []string{"Name", "Score", "Passed", "Alice", "90", "Yes", "Carol", "40", "No"} {
		if !strings.Contains(md, want) {
			t.Errorf("expected %q in output, got:\n%s", want, md)
		}
	}
	if !strings.Contains(md, "Sheet1") && !strings.Contains(md, "sample") {
		t.Logf("sheet name heading not found verbatim (non-fatal, LibreOffice may name it differently); output:\n%s", md)
	}
}

func TestExtractDOCX_InvalidFile(t *testing.T) {
	if _, err := ExtractDOCX([]byte("not a zip at all")); err == nil {
		t.Fatal("expected an error for non-zip input, got nil")
	}
}

func TestExtractXLSX_InvalidFile(t *testing.T) {
	if _, err := ExtractXLSX([]byte("not a zip at all")); err == nil {
		t.Fatal("expected an error for non-zip input, got nil")
	}
}

func TestExtractPPTX_InvalidFile(t *testing.T) {
	if _, err := ExtractPPTX([]byte("not a zip at all")); err == nil {
		t.Fatal("expected an error for non-zip input, got nil")
	}
}

func TestExtract_Dispatch(t *testing.T) {
	if _, ok, _ := Extract("notes.txt", []byte("hi")); ok {
		t.Fatal("expected ok=false for an unsupported extension")
	}
	if _, ok, _ := Extract("report.DOCX", readTestdata(t, "sample.docx")); !ok {
		t.Fatal("expected ok=true for .DOCX (case-insensitive)")
	}
}

// --- Hand-built PPTX fixture -----------------------------------------------
// LibreOffice headless has no direct outline/markdown -> pptx export filter,
// so this synthetic minimal-but-schema-valid deck (2 slides, a title
// placeholder, a body placeholder, and a table on slide 2) is built in code
// instead of checked in as a binary. It still exercises the real OOXML
// element names (p:sldIdLst/p:sldId/r:id resolution, p:ph type="title",
// a:tbl) - not a shortcut around them.

func buildTestPPTX(t *testing.T) []byte {
	t.Helper()

	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
<Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,

		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,

		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<p:sldIdLst>
<p:sldId id="256" r:id="rIdSlide2"/>
<p:sldId id="257" r:id="rIdSlide1"/>
</p:sldIdLst>
</p:presentation>`,

		// Deliberately out-of-filename-order relationship targets: rIdSlide2
		// (listed first in sldIdLst) points at slide1.xml and vice versa, so
		// a test that only sorted by filename would get the order backwards.
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rIdSlide2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
<Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,

		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>
<p:sp>
<p:nvSpPr><p:nvPr><p:ph type="ctrTitle"/></p:nvPr></p:nvSpPr>
<p:txBody><a:p><a:r><a:t>Welcome Slide</a:t></a:r></a:p></p:txBody>
</p:sp>
<p:sp>
<p:nvSpPr><p:nvPr/></p:nvSpPr>
<p:txBody><a:p><a:r><a:t>This should come second</a:t></a:r></a:p></p:txBody>
</p:sp>
</p:spTree></p:cSld>
</p:sld>`,

		"ppt/slides/slide2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>
<p:sp>
<p:nvSpPr><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>
<p:txBody><a:p><a:r><a:t>Data Slide</a:t></a:r></a:p></p:txBody>
</p:sp>
<p:graphicFrame>
<a:graphic><a:graphicData>
<a:tbl>
<a:tr><a:tc><a:txBody><a:p><a:r><a:t>Col A</a:t></a:r></a:p></a:txBody></a:tc><a:tc><a:txBody><a:p><a:r><a:t>Col B</a:t></a:r></a:p></a:txBody></a:tc></a:tr>
<a:tr><a:tc><a:txBody><a:p><a:r><a:t>1</a:t></a:r></a:p></a:txBody></a:tc><a:tc><a:txBody><a:p><a:r><a:t>2</a:t></a:r></a:p></a:txBody></a:tc></a:tr>
</a:tbl>
</a:graphicData></a:graphic>
</p:graphicFrame>
</p:spTree></p:cSld>
</p:sld>`,
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractPPTX_SlideOrderAndContent(t *testing.T) {
	md, err := ExtractPPTX(buildTestPPTX(t))
	if err != nil {
		t.Fatalf("ExtractPPTX: %v", err)
	}

	welcomeIdx := strings.Index(md, "Welcome Slide")
	dataIdx := strings.Index(md, "Data Slide")
	if welcomeIdx == -1 || dataIdx == -1 {
		t.Fatalf("expected both slide titles in output, got:\n%s", md)
	}
	// sldIdLst lists rIdSlide2 (-> slide2.xml, "Data Slide") before rIdSlide1
	// (-> slide1.xml, "Welcome Slide") - the reverse of filename order. Output
	// must follow sldIdLst order, proving the extractor resolves the rels
	// chain rather than just sorting slideN.xml by N.
	if dataIdx > welcomeIdx {
		t.Errorf("slide order wrong: expected 'Data Slide' (sldIdLst order) before 'Welcome Slide' (filename order would say the opposite), got:\n%s", md)
	}

	if !strings.Contains(md, "## Slide 1") || !strings.Contains(md, "## Slide 2") {
		t.Errorf("expected slide-numbered headings, got:\n%s", md)
	}
	if !strings.Contains(md, "### Welcome Slide") {
		t.Errorf("expected title placeholder promoted to heading, got:\n%s", md)
	}
	if !strings.Contains(md, "- This should come second") {
		t.Errorf("expected non-title text rendered as a bullet, got:\n%s", md)
	}
	if !strings.Contains(md, "Col A") || !strings.Contains(md, "Col B") {
		t.Errorf("expected table content from graphicFrame, got:\n%s", md)
	}
}

func TestExtractPPTX_EmptyDeckIsErrEmpty(t *testing.T) {
	files := map[string]string{
		"[Content_Types].xml":             `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"ppt/presentation.xml":            `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()

	if _, err := ExtractPPTX(buf.Bytes()); err == nil {
		t.Fatal("expected an error for a deck with no slides")
	}
}
