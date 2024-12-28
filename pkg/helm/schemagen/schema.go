// Copyright (c) 2023 dadav. MIT License

package schemagen

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Generator string

const (
	AutoGenerator      Generator = "AUTO"
	ValuesGenerator    Generator = "VALUE-INFERENCE"
	URLGenerator       Generator = "URL"
	PathGenerator      Generator = "PATH"
	LocalPathGenerator Generator = "LOCAL-PATH"
	NoGenerator        Generator = "NONE"
)

var (
	Generators = []Generator{
		AutoGenerator,
		ValuesGenerator,
		URLGenerator,
		PathGenerator,
		LocalPathGenerator,
		NoGenerator,
	}
	GeneratorEnum = []interface{}{
		AutoGenerator,
		ValuesGenerator,
		URLGenerator,
		PathGenerator,
		LocalPathGenerator,
		NoGenerator,
	}
)

var (
	ValuesOrSchemaRegexp = regexp.MustCompile(`.*values.*\.(ya?ml|json)$`)
	ValuesRegexp         = regexp.MustCompile(`.*values.*\.ya?ml$`)
)

// NewPathFilter returns a filter function that returns true for files relevant
// to the given [Generator].
func NewPathFilter(g Generator, path string) func(string) bool {
	switch g {
	case URLGenerator, NoGenerator:
		break
	case AutoGenerator:
		return func(f string) bool {
			return ValuesOrSchemaRegexp.MatchString(f)
		}
	case ValuesGenerator:
		return func(f string) bool {
			return ValuesRegexp.MatchString(f)
		}
	case PathGenerator, LocalPathGenerator:
		return func(f string) bool {
			file1, err := os.Stat(f)
			if err != nil {
				return false
			}
			file2, err := os.Stat(path)
			if err != nil {
				return false
			}
			return os.SameFile(file1, file2)
		}
	}
	return func(_ string) bool {
		return false
	}
}

func IsYAMLFile(f string) bool {
	return strings.HasSuffix(f, ".yaml") || strings.HasSuffix(f, ".yml")
}

func IsJSONFile(f string) bool {
	return strings.HasSuffix(f, ".json")
}

func GetSchemaFromURL(schemaURL string) ([]byte, error) {
	schemaNetURL, err := url.Parse(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	schema, err := http.DefaultClient.Do(&http.Request{
		Method: http.MethodGet,
		URL:    schemaNetURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed http request: %w", err)
	}

	jsBytes, err := io.ReadAll(schema.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	if err := schema.Body.Close(); err != nil {
		return nil, fmt.Errorf("failed to close body: %w", err)
	}

	return jsBytes, nil
}

func GetSchemaFromFile(schemaPath string) ([]byte, error) {
	jsBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return jsBytes, nil
}
