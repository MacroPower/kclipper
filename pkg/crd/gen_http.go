package crd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/macropower/kclipper/pkg/kube"
)

// HTTPDoer is the interface for making HTTP requests.
// See [*net/http.Client] for an implementation.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// FromURLs reads CRDs from the given HTTP URLs and returns the corresponding
// []kube.Object representations.
func FromURLs(ctx context.Context, httpClient HTTPDoer, crdURLs ...*url.URL) ([]kube.Object, error) {
	if len(crdURLs) == 0 {
		return nil, errors.New("no urls provided")
	}

	crds := []kube.Object{}
	for _, crdURL := range crdURLs {
		c, err := FromURL(ctx, httpClient, crdURL)
		if err != nil {
			return nil, fmt.Errorf("read CRDs from %s: %w", crdURL.String(), err)
		}

		crds = append(crds, c...)
	}

	return crds, nil
}

// FromURL reads CRDs from the given HTTP URL and returns the corresponding
// []kube.Object representation.
func FromURL(ctx context.Context, httpClient HTTPDoer, crdURL *url.URL) ([]kube.Object, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crdURL.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	schema, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	defer func() {
		err := schema.Body.Close()
		if err != nil {
			slog.Error("close http response body",
				slog.String("url", crdURL.String()),
				slog.Any("err", err),
			)
		}
	}()

	return FromReader(schema.Body)
}
