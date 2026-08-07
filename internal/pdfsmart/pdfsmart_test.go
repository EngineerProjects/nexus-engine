package pdfsmart

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KPO-Tech/seshat/internal/docling"
)

// Fixtures are copied from internal/pdftext's own testdata (already
// validated there): text_layer.pdf is a real multi-element text-layer PDF
// with no embedded images; scanned.pdf is a real pdfcpu-built PDF whose
// single page is nothing but an embedded screenshot image - exactly the
// "has an embedded image XObject" case this package routes to docling.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "pdftext", "testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return data
}

func TestConvert_TextOnlyPDFExtractsNativelyWithNilDoclingClient(t *testing.T) {
	t.Parallel()
	result, ok, err := Convert(context.Background(), readTestdata(t, "text_layer.pdf"), nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !ok {
		t.Fatalf("expected a text-only PDF (no embedded images) to succeed with no docling client, got pages: %+v", result.Pages)
	}
	if !strings.Contains(result.Markdown, "Sample Report") {
		t.Fatalf("expected extracted content, got:\n%s", result.Markdown)
	}
	for _, p := range result.Pages {
		if p.Source != PageSourceNative {
			t.Errorf("expected every page to be extracted natively, page %d got source %q", p.Page, p.Source)
		}
	}
	if result.DoclingPageCount() != 0 {
		t.Errorf("expected 0 pages routed to docling, got %d", result.DoclingPageCount())
	}
}

func TestConvert_ImagePDFFailsCleanlyWithNilDoclingClient(t *testing.T) {
	t.Parallel()
	_, ok, err := Convert(context.Background(), readTestdata(t, "scanned.pdf"), nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	// The page has an embedded image, so it needs docling - with no
	// client available, ok must be false (the safety contract: never
	// return a result missing a page docling would have covered).
	if ok {
		t.Fatal("expected ok=false when a page needing docling has no docling client to use")
	}
}

func TestConvert_ImagePDFUsesDoclingWhenAvailable(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/convert/file":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success","document":{"md_content":"# This came from docling for this page","pages":[1]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := docling.NewClient(server.URL)
	result, ok, err := Convert(context.Background(), readTestdata(t, "scanned.pdf"), client)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !ok {
		t.Fatalf("expected success once docling is available for the image page, got pages: %+v", result.Pages)
	}
	if !strings.Contains(result.Markdown, "This came from docling for this page") {
		t.Fatalf("expected docling's markdown in the result, got:\n%s", result.Markdown)
	}
	if result.DoclingPageCount() == 0 {
		t.Error("expected at least one page to be routed to docling for an image-only PDF")
	}
}

func TestConvert_GarbledDoclingOutputForAnImagePageFailsCleanly(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/convert/file":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success","document":{"md_content":"(cid:12)(cid:47)(cid:8)(cid:91)","pages":[1]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := docling.NewClient(server.URL)
	_, ok, err := Convert(context.Background(), readTestdata(t, "scanned.pdf"), client)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if ok {
		t.Fatal("expected garbled docling output on the only page to be rejected, not accepted as success")
	}
}
