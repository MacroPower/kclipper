package crd_test

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/MacroPower/kclipper/pkg/crd"
)

// MockHTTPClient implements the HTTPDoer interface for testing
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

// readTestFile reads the content of a test file from the testdata directory
func readTestFile(t *testing.T, filename string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err, "Failed to read test file: %s", filename)

	return string(content)
}

func TestHTTPGenerator_FromURL(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupMock func(t *testing.T) *MockHTTPClient
		validate  func(t *testing.T, crds []*unstructured.Unstructured, err error)
		url       string
	}{
		"successful request": {
			url: "https://example.com/crd.yaml",
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				validCRDContent := readTestFile(t, "valid-crd.yaml")

				return &MockHTTPClient{
					DoFunc: func(_ *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(validCRDContent)),
						}, nil
					},
				}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 1)
				assert.Equal(t, "CustomResourceDefinition", crds[0].GetKind())
				assert.Equal(t, "apiextensions.k8s.io/v1", crds[0].GetAPIVersion())
				assert.Equal(t, "widgets.example.com", crds[0].GetName())
			},
		},
		"http request error": {
			url: "https://example.com/error",
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				return &MockHTTPClient{
					DoFunc: func(_ *http.Request) (*http.Response, error) {
						return nil, errors.New("connection refused")
					},
				}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed http request")
				assert.Contains(t, err.Error(), "connection refused")
				assert.Nil(t, crds)
			},
		},
		"invalid yaml": {
			url: "https://example.com/invalid.yaml",
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				invalidYAMLContent := readTestFile(t, "invalid-yaml.yaml")

				return &MockHTTPClient{
					DoFunc: func(_ *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(invalidYAMLContent)),
						}, nil
					},
				}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Nil(t, crds)
			},
		},
		"non-crd yaml": {
			url: "https://example.com/not-crd.yaml",
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				noCRDsContent := readTestFile(t, "no-crds.yaml")

				return &MockHTTPClient{
					DoFunc: func(_ *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(noCRDsContent)),
						}, nil
					},
				}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				assert.Empty(t, crds)
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockClient := tc.setupMock(t)
			generator := crd.NewHTTPGenerator(mockClient)

			parsedURL, err := url.Parse(tc.url)
			require.NoError(t, err)

			crds, err := generator.FromURL(t.Context(), parsedURL)
			tc.validate(t, crds, err)
		})
	}
}

func TestHTTPGenerator_FromURLs(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		setupMock func(t *testing.T) *MockHTTPClient
		validate  func(t *testing.T, crds []*unstructured.Unstructured, err error)
		urls      []string
	}{
		"no urls": {
			urls: []string{},
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				return &MockHTTPClient{}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "no urls provided")
				assert.Nil(t, crds)
			},
		},
		"single url": {
			urls: []string{"https://example.com/crd.yaml"},
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				validCRDContent := readTestFile(t, "valid-crd.yaml")

				return &MockHTTPClient{
					DoFunc: func(_ *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(validCRDContent)),
						}, nil
					},
				}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 1)
				assert.Equal(t, "widgets.example.com", crds[0].GetName())
			},
		},
		"multiple urls": {
			urls: []string{
				"https://example.com/crd1.yaml",
				"https://example.com/crd2.yaml",
			},
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				validCRDContent := readTestFile(t, "valid-crd.yaml")
				multipleCRDsContent := readTestFile(t, "multiple-crds.yaml")

				return &MockHTTPClient{
					DoFunc: func(req *http.Request) (*http.Response, error) {
						switch req.URL.String() {
						case "https://example.com/crd1.yaml":
							return &http.Response{
								StatusCode: http.StatusOK,
								Body:       io.NopCloser(strings.NewReader(validCRDContent)),
							}, nil
						case "https://example.com/crd2.yaml":
							return &http.Response{
								StatusCode: http.StatusOK,
								Body:       io.NopCloser(strings.NewReader(multipleCRDsContent)),
							}, nil
						default:
							return nil, errors.New("unexpected URL")
						}
					},
				}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 3)

				// Extract all CRD names for easier validation
				names := make([]string, len(crds))
				for i, c := range crds {
					names[i] = c.GetName()
				}

				// Check that all expected CRDs are present
				assert.Contains(t, names, "widgets.example.com")
				assert.Contains(t, names, "gadgets.example.com")

				// Count occurrences of widgets.example.com (should appear twice - once from each file)
				widgetCount := 0
				for _, name := range names {
					if name == "widgets.example.com" {
						widgetCount++
					}
				}
				assert.Equal(t, 2, widgetCount)
			},
		},
		"one url with error": {
			urls: []string{
				"https://example.com/crd1.yaml",
				"https://example.com/error.yaml",
			},
			setupMock: func(t *testing.T) *MockHTTPClient {
				t.Helper()

				validCRDContent := readTestFile(t, "valid-crd.yaml")

				return &MockHTTPClient{
					DoFunc: func(req *http.Request) (*http.Response, error) {
						switch req.URL.String() {
						case "https://example.com/crd1.yaml":
							return &http.Response{
								StatusCode: http.StatusOK,
								Body:       io.NopCloser(strings.NewReader(validCRDContent)),
							}, nil
						case "https://example.com/error.yaml":
							return nil, errors.New("connection refused")
						default:
							return nil, errors.New("unexpected URL")
						}
					},
				}
			},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to read CRDs from")
				assert.Contains(t, err.Error(), "connection refused")
				assert.Nil(t, crds)
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockClient := tc.setupMock(t)
			generator := crd.NewHTTPGenerator(mockClient)

			var urls []*url.URL
			for _, u := range tc.urls {
				parsedURL, err := url.Parse(u)
				require.NoError(t, err)
				urls = append(urls, parsedURL)
			}

			crds, err := generator.FromURLs(t.Context(), urls...)
			tc.validate(t, crds, err)
		})
	}
}

func TestDefaultHTTPGenerator(t *testing.T) {
	t.Parallel()

	// Ensure the default generator is properly initialized
	assert.NotNil(t, crd.DefaultHTTPGenerator)
	assert.NotNil(t, crd.DefaultHTTPGenerator.HTTPClient)
}
