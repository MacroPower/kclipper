package jsonschema

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	jsv6 "github.com/santhosh-tekuri/jsonschema/v6"
)

// DefaultReaderGenerator is an opinionated [ReaderGenerator].
var DefaultReaderGenerator = NewReaderGenerator()

var _ Generator = DefaultReaderGenerator

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
	multiErr := fmt.Errorf("could not read JSON Schema from any of the provided paths: [\n%s]", pathErrMsgs)

	return nil, fmt.Errorf("error generating JSON Schema: %w", multiErr)
}

// fromPath reads a JSON Schema from the given file path or URL and returns the
// corresponding []byte representation.
func (g *ReaderGenerator) fromPath(path string) ([]byte, error) {
	schemaPath, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
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
	jsBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return g.FromReader(bytes.NewReader(jsBytes))
}

func (g *ReaderGenerator) FromURL(schemaURL *url.URL) ([]byte, error) {
	schema, err := http.DefaultClient.Do(&http.Request{
		Method: http.MethodGet,
		URL:    schemaURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed http request: %w", err)
	}
	defer schema.Body.Close()

	return g.FromReader(schema.Body)
}

func (g *ReaderGenerator) FromReader(r io.Reader) ([]byte, error) {
	jsBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	return g.FromData(jsBytes)
}

func (g *ReaderGenerator) FromData(data []byte) ([]byte, error) {
	compiler := jsv6.NewCompiler()
	if err := compiler.AddResource("schema.json", data); err != nil {
		return nil, fmt.Errorf("invalid JSON Schema: %w", err)
	}

	return data, nil
}
