package docx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

// readDocumentXML extracts word/document.xml's raw contents from a DOCX's
// zip container.
func readDocumentXML(data []byte) (string, error) {
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
	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("failed to read word/document.xml: %w", err)
	}
	return string(raw), nil
}

// replaceDocumentXML rewrites a DOCX zip archive with word/document.xml's
// content replaced, copying every other part through unchanged (headers,
// footers, styles, media, relationships - none of that should be touched by
// a text edit to the body).
func replaceDocumentXML(data []byte, newDocumentXML string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid DOCX (zip open failed): %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	replaced := false
	for _, f := range zr.File {
		w, err := zw.Create(f.Name)
		if err != nil {
			return nil, fmt.Errorf("zip create %s: %w", f.Name, err)
		}
		if f.Name == "word/document.xml" {
			if _, err := w.Write([]byte(newDocumentXML)); err != nil {
				return nil, fmt.Errorf("write %s: %w", f.Name, err)
			}
			replaced = true
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}
		_, copyErr := io.Copy(w, rc)
		rc.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("copy %s: %w", f.Name, copyErr)
		}
	}
	if !replaced {
		return nil, fmt.Errorf("not a valid DOCX: missing word/document.xml")
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize zip: %w", err)
	}
	return buf.Bytes(), nil
}

func findZipFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}
