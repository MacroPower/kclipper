package jsonschema

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

var (
	// DefaultReaderGenerator is an opinionated [ReaderGenerator].
	DefaultReaderGenerator = NewReaderGenerator()

	_ FileGenerator = DefaultReaderGenerator
)

// ReaderGenerator reads a JSON Schema from a given location and returns
// corresponding []byte representations.
type ReaderGenerator struct{}

// NewReaderGenerator creates a new [ReaderGenerator].
func NewReaderGenerator() *ReaderGenerator {
	return &ReaderGenerator{}
}

// FromPaths reads a JSON Schema from at least one of the given file paths or
// URLs and returns the corresponding []byte representation. It will return an
// error only if none of the paths provide a valid JSON Schema.
func (g *ReaderGenerator) FromPaths(paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, errors.New("no paths provided")
	}

	if len(paths) == 1 {
		return g.fromPath(paths[0])
	}

	pathErrs := map[string]error{}

	for _, path := range paths {
		jsBytes, err := g.fromPath(path)
		if err == nil {
			return jsBytes, nil
		}

		pathErrs[path] = err
	}

	pathErrMsgs := []string{}
	for path, err := range pathErrs {
		pathErrMsgs = append(pathErrMsgs, fmt.Sprintf("\t%s: %s\n", path, err))
	}

	multiErr := fmt.Errorf("could not read JSON Schema from any of the provided paths:\n%s", pathErrMsgs)

	return nil, fmt.Errorf("generate JSON Schema: %w", multiErr)
}

// fromPath reads a JSON Schema from the given file path or URL and returns the
// corresponding []byte representation.
func (g *ReaderGenerator) fromPath(path string) ([]byte, error) {
	schemaPath, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}

	switch schemaPath.Scheme {
	case "http", "https":
		return g.FromURL(schemaPath)
	case "":
		return g.FromFile(schemaPath.Path)
	}

	return nil, fmt.Errorf("unsupported scheme: %s", schemaPath.Scheme)
}

func (g *ReaderGenerator) FromFile(path string) ([]byte, error) {
	//nolint:gosec // G304 not relevant for client-side generation.
	jsBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	baseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	return g.FromData(jsBytes, baseDir)
}

func (g *ReaderGenerator) FromURL(schemaURL *url.URL) ([]byte, error) {
	schema, err := http.DefaultClient.Do(&http.Request{
		Method: http.MethodGet,
		URL:    schemaURL,
	})
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	defer func() {
		err = schema.Body.Close()
		if err != nil {
			slog.Error("close http response body",
				slog.String("url", schemaURL.String()),
				slog.Any("err", err),
			)
		}
	}()

	return g.FromReader(schema.Body, "")
}

func (g *ReaderGenerator) FromReader(r io.Reader, refBasePath string) ([]byte, error) {
	jsBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	return g.FromData(jsBytes, refBasePath)
}

func (g *ReaderGenerator) FromData(data []byte, refBasePath string) ([]byte, error) {
	hs, err := unmarshalHelmSchema(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	err = hs.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	err = handleSchemaRefs(hs, refBasePath)
	if err != nil {
		return nil, fmt.Errorf("handle schema refs: %w", err)
	}

	err = hs.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}

	if len(hs.Properties) == 0 {
		return nil, errors.New("empty schema")
	}

	resolvedData, err := hs.ToJson()
	if err != nil {
		return nil, fmt.Errorf("convert schema to JSON: %w", err)
	}

	return resolvedData, nil
}
