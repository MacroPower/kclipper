package crd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DefaultHTTPGenerator is an opinionated [HTTPGenerator].
var DefaultHTTPGenerator = NewHTTPGenerator(http.DefaultClient)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ReaderGenerator reads CRDs from a given location and returns
// corresponding []*unstructured.Unstructured representations.
type HTTPGenerator struct {
	*ReaderGenerator
	HTTPClient HTTPDoer
}

func NewHTTPGenerator(httpClient HTTPDoer) *HTTPGenerator {
	return &HTTPGenerator{
		HTTPClient:      httpClient,
		ReaderGenerator: NewReaderGenerator(),
	}
}

func (g *HTTPGenerator) FromURLs(ctx context.Context, crdURLs ...*url.URL) ([]*unstructured.Unstructured, error) {
	if len(crdURLs) == 0 {
		return nil, errors.New("no urls provided")
	}

	crds := []*unstructured.Unstructured{}
	for _, crdURL := range crdURLs {
		c, err := g.FromURL(ctx, crdURL)
		if err != nil {
			return nil, fmt.Errorf("failed to read CRDs from %s: %w", crdURL.String(), err)
		}
		crds = append(crds, c...)
	}

	return crds, nil
}

func (g *HTTPGenerator) FromURL(ctx context.Context, crdURL *url.URL) ([]*unstructured.Unstructured, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crdURL.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	schema, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed http request: %w", err)
	}
	defer func() {
		if err := schema.Body.Close(); err != nil {
			slog.Error("failed to close http response body",
				slog.String("url", crdURL.String()),
				slog.Any("err", err),
			)
		}
	}()

	return g.FromReader(schema.Body)
}
