package providers

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// A trimmed but structurally real fragment of html.duckduckgo.com/html/'s
// response shape (div.result / a.result__a / .result__snippet /
// .result__url), including a click-tracked link (the //duckduckgo.com/l/
// wrapper) and an ad result that must be skipped.
const sampleDuckDuckGoHTML = `<!DOCTYPE html>
<html><body>
<div class="results">
  <div class="result results_links results_links_deep web-result result--ad">
    <a class="result__a" href="https://ads.example.com/click">Sponsored Result</a>
    <a class="result__snippet">Buy now</a>
  </div>
  <div class="result results_links results_links_deep web-result">
    <a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2Fdoc%2F&amp;rut=abc123">The Go Programming Language Documentation</a>
    <a class="result__snippet">Official documentation for the Go programming language.</a>
    <span class="result__url">golang.org/doc</span>
  </div>
  <div class="result results_links results_links_deep web-result">
    <a class="result__a" href="https://pkg.go.dev/net/http">net/http package - Go Packages</a>
    <a class="result__snippet">Package http provides HTTP client and server implementations.</a>
    <span class="result__url">pkg.go.dev/net/http</span>
  </div>
</div>
</body></html>`

func TestDuckDuckGoProvider_IsConfigured(t *testing.T) {
	p := NewDuckDuckGoProvider()
	if !p.IsConfigured() {
		t.Error("DuckDuckGoProvider must always report IsConfigured=true - it needs no API key")
	}
}

func TestDuckDuckGoProvider_ParsesResultsAndResolvesRedirects(t *testing.T) {
	provider := &DuckDuckGoProvider{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("expected POST, got %s", req.Method)
				}
				if req.URL.String() != "https://html.duckduckgo.com/html/" {
					t.Fatalf("unexpected URL: %s", req.URL.String())
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if !strings.Contains(string(body), "q=golang+http+client") {
					t.Fatalf("expected query in body, got %s", body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(sampleDuckDuckGoHTML)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	output, err := provider.Search(SearchInput{Query: "golang http client"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if output.ProviderName != "duckduckgo" {
		t.Errorf("unexpected provider name: %q", output.ProviderName)
	}
	if len(output.Hits) != 2 {
		t.Fatalf("expected 2 hits (ad excluded), got %d: %+v", len(output.Hits), output.Hits)
	}

	first := output.Hits[0]
	if first.Title != "The Go Programming Language Documentation" {
		t.Errorf("unexpected title: %q", first.Title)
	}
	if first.URL != "https://golang.org/doc/" {
		t.Errorf("expected the redirect wrapper to be resolved to the real URL, got %q", first.URL)
	}
	if first.Description != "Official documentation for the Go programming language." {
		t.Errorf("unexpected description: %q", first.Description)
	}
	if first.Source != "golang.org/doc" {
		t.Errorf("unexpected source: %q", first.Source)
	}

	second := output.Hits[1]
	if second.URL != "https://pkg.go.dev/net/http" {
		t.Errorf("expected an unwrapped href to pass through unchanged, got %q", second.URL)
	}

	for _, hit := range output.Hits {
		if strings.Contains(hit.Title, "Sponsored") || strings.Contains(hit.URL, "ads.example.com") {
			t.Errorf("ad result should have been excluded, got %+v", hit)
		}
	}
}

func TestDuckDuckGoProvider_ErrorStatus(t *testing.T) {
	provider := &DuckDuckGoProvider{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}
	if _, err := provider.Search(SearchInput{Query: "x"}); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestResolveDuckDuckGoRedirect(t *testing.T) {
	cases := []struct {
		name string
		href string
		want string
	}{
		{
			name: "wrapped redirect",
			href: "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpage&rut=x",
			want: "https://example.com/page",
		},
		{
			name: "unwrapped direct link",
			href: "https://example.com/direct",
			want: "https://example.com/direct",
		},
		{
			name: "malformed url falls back unchanged",
			href: "://not a url",
			want: "://not a url",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDuckDuckGoRedirect(tc.href); got != tc.want {
				t.Errorf("resolveDuckDuckGoRedirect(%q) = %q, want %q", tc.href, got, tc.want)
			}
		})
	}
}

func TestAllProviders_IncludesDuckDuckGoAsAlwaysConfiguredFallback(t *testing.T) {
	found := false
	for _, p := range AllProviders() {
		if p.Name() == "duckduckgo" {
			found = true
			if !p.IsConfigured() {
				t.Error("duckduckgo provider in AllProviders() must be configured (no key needed)")
			}
		}
	}
	if !found {
		t.Fatal(`"duckduckgo" not found in AllProviders()`)
	}
	// With no other provider configured (no env keys set in a clean test
	// process), IsProviderAvailable must still be true - the whole point of
	// adding this provider is that web_search works with zero setup.
	if !IsProviderAvailable() {
		t.Error("expected IsProviderAvailable() to be true thanks to the always-configured duckduckgo fallback")
	}
}
