package providers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DuckDuckGoProvider scrapes DuckDuckGo's lightweight HTML endpoint
// (html.duckduckgo.com/html/) directly - no API key, no self-hosted
// service, nothing to configure. It exists so web_search still works out of
// the box with zero setup, instead of being silently disabled until a user
// configures one of Tavily/Exa/Jina/LangSearch/SearXNG (see registry.go's
// autoProviderPriority - this is deliberately last in the chain: an
// HTML-scraped result is less reliable than a real search API and more
// exposed to upstream markup changes or rate limiting).
type DuckDuckGoProvider struct {
	httpClient *http.Client
}

// NewDuckDuckGoProvider creates the scraping provider. No API key needed.
func NewDuckDuckGoProvider() *DuckDuckGoProvider {
	return &DuckDuckGoProvider{httpClient: &http.Client{Timeout: 20 * time.Second}}
}

func (p *DuckDuckGoProvider) Name() string {
	return "duckduckgo"
}

// IsConfigured is unconditionally true - that's the entire point of this
// provider existing: it needs no key and no setup.
func (p *DuckDuckGoProvider) IsConfigured() bool {
	return true
}

// Search POSTs to DuckDuckGo's no-JS HTML results page and parses the
// response with goquery. This endpoint is DuckDuckGo's own lightweight
// client-facing page (not an internal/private API), and unlike Bing/Sogou/
// Startpage doesn't gate results behind a CAPTCHA or session token in
// normal use.
func (p *DuckDuckGoProvider) Search(input SearchInput) (ProviderOutput, error) {
	start := time.Now()

	form := url.Values{}
	form.Set("q", input.Query)

	req, err := http.NewRequestWithContext(input.ctx(), http.MethodPost,
		"https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return ProviderOutput{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ProviderOutput{}, fmt.Errorf("duckduckgo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ProviderOutput{}, fmt.Errorf("duckduckgo returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ProviderOutput{}, fmt.Errorf("failed to parse duckduckgo response: %w", err)
	}

	hits := make([]SearchHit, 0, 10)
	doc.Find("div.result").Each(func(_ int, s *goquery.Selection) {
		if s.HasClass("result--ad") {
			return
		}
		link := s.Find("a.result__a").First()
		if link.Length() == 0 {
			return
		}
		href, exists := link.Attr("href")
		if !exists || strings.TrimSpace(href) == "" {
			return
		}

		hits = append(hits, SearchHit{
			Title:       strings.TrimSpace(link.Text()),
			URL:         resolveDuckDuckGoRedirect(href),
			Description: strings.TrimSpace(s.Find(".result__snippet").First().Text()),
			Source:      strings.TrimSpace(s.Find(".result__url").First().Text()),
		})
	})

	return ProviderOutput{
		Hits:            hits,
		ProviderName:    "duckduckgo",
		DurationSeconds: time.Since(start).Seconds(),
	}, nil
}

// resolveDuckDuckGoRedirect extracts the real target URL from DuckDuckGo's
// click-tracking redirect wrapper
// (//duckduckgo.com/l/?uddg=<url-encoded-target>&...), falling back to href
// unchanged if it isn't wrapped that way (DuckDuckGo doesn't always wrap
// results, depending on the result type).
func resolveDuckDuckGoRedirect(href string) string {
	normalized := href
	if strings.HasPrefix(normalized, "//") {
		normalized = "https:" + normalized
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return href
	}
	if !strings.Contains(u.Host, "duckduckgo.com") || u.Path != "/l/" {
		return href
	}
	target := u.Query().Get("uddg")
	if target == "" {
		return href
	}
	if decoded, err := url.QueryUnescape(target); err == nil {
		return decoded
	}
	return target
}
