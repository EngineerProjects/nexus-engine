package docling

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_DefaultsTo120SecondTimeout(t *testing.T) {
	c := NewClient("http://localhost:5001")
	if c.httpClient.Timeout != 120*time.Second {
		t.Fatalf("expected default timeout of 120s, got %v", c.httpClient.Timeout)
	}
}

func TestNewClient_WithTimeoutOverridesTheDefault(t *testing.T) {
	c := NewClient("http://localhost:5001", WithTimeout(10*time.Minute))
	if c.httpClient.Timeout != 10*time.Minute {
		t.Fatalf("expected WithTimeout to override the default, got %v", c.httpClient.Timeout)
	}
}

// A slow docling-serve response that would exceed the default 120s timeout
// must still succeed once WithTimeout raises the budget - this is the whole
// point of making it configurable: large/scan-heavy documents on a
// CPU-only deployment can genuinely take that long.
func TestConvertBytes_WithTimeoutAllowsASlowResponseToSucceed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/convert/file":
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success","document":{"md_content":"# slow but done","pages":[1]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, WithTimeout(200*time.Millisecond))
	result, err := c.ConvertBytes(context.Background(), []byte("dummy pdf bytes"), "report.pdf")
	if err != nil {
		t.Fatalf("ConvertBytes: %v", err)
	}
	if result.Markdown != "# slow but done" {
		t.Fatalf("unexpected markdown: %q", result.Markdown)
	}
}
