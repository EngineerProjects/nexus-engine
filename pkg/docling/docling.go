package docling

import (
	"time"

	internaldocling "github.com/KPO-Tech/seshat/internal/docling"
)

type (
	Client           = internaldocling.Client
	ConversionResult = internaldocling.ConversionResult
	ExtractedImage   = internaldocling.ExtractedImage
	Option           = internaldocling.Option
)

// NewClient creates a client pointing at a docling-serve base URL.
// baseURL is typically "http://localhost:5001". Defaults to a 120s
// per-request timeout - pass WithTimeout to override it for deployments
// that expect large or scan-heavy documents.
func NewClient(baseURL string, opts ...Option) *Client {
	return internaldocling.NewClient(baseURL, opts...)
}

// WithTimeout overrides the default 120s per-request HTTP timeout.
func WithTimeout(d time.Duration) Option {
	return internaldocling.WithTimeout(d)
}
