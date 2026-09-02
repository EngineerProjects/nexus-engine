package nodes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateURLForSSRFBlocksPrivateAndMetadataHosts(t *testing.T) {
	cases := []string{
		"http://localhost/",
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:8080/",
		"ftp://example.com/",
	}
	for _, url := range cases {
		if err := validateURLForSSRF(url); err == nil {
			t.Errorf("expected %q to be blocked", url)
		}
	}
}

func TestValidateURLForSSRFAllowsPublicHTTPS(t *testing.T) {
	if err := validateURLForSSRF("https://example.com/path"); err != nil {
		t.Fatalf("expected example.com to be allowed, got %v", err)
	}
}

func TestHTTPRequestValidateParametersRequiresURL(t *testing.T) {
	node := NewHTTPRequest()
	if err := node.ValidateParameters(map[string]any{}); err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestHTTPRequestValidateParametersEnforcesSSRFGuard(t *testing.T) {
	node := NewHTTPRequest()
	if err := node.ValidateParameters(map[string]any{"url": "http://169.254.169.254/"}); err == nil {
		t.Fatal("expected the real SSRF guard to reject a metadata URL")
	}
}

func TestHTTPRequestExecuteParsesJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Test"); got != "yes" {
			t.Errorf("expected header X-Test=yes, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"n":3}`))
	}))
	defer srv.Close()

	node := NewHTTPRequest()
	node.checkSSRF = func(string) error { return nil } // httptest binds to 127.0.0.1, itself a blocked address

	output, err := node.Execute(context.Background(), nil, nil, map[string]any{
		"url":     srv.URL,
		"method":  "GET",
		"headers": map[string]any{"X-Test": "yes"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	items := output.Ports["main"]
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["status"] != 200 {
		t.Fatalf("expected status 200, got %#v", items[0]["status"])
	}
	parsed, ok := items[0]["json"].(map[string]any)
	if !ok || parsed["ok"] != true {
		t.Fatalf("expected parsed json body, got %#v", items[0]["json"])
	}
}

func TestHTTPRequestExecuteRejectsBlockedURL(t *testing.T) {
	node := NewHTTPRequest()
	_, err := node.Execute(context.Background(), nil, nil, map[string]any{"url": "http://169.254.169.254/"})
	if err == nil {
		t.Fatal("expected Execute to enforce the SSRF guard even if ValidateParameters was skipped")
	}
}
