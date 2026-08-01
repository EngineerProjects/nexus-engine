package providers

import "testing"

func TestNewDuckDuckGoProvider_PublicWrapper(t *testing.T) {
	p := NewDuckDuckGoProvider()
	if p == nil {
		t.Fatal("expected a non-nil provider")
	}
}

func TestProviderModeDuckDuckGo_PublicWrapper(t *testing.T) {
	if ProviderModeDuckDuckGo != "duckduckgo" {
		t.Errorf("ProviderModeDuckDuckGo = %q, want %q", ProviderModeDuckDuckGo, "duckduckgo")
	}
}
