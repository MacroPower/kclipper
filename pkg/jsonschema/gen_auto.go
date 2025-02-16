package jsonschema

import (
	"errors"
	"fmt"
)

var (
	DefaultAutoGenerator = NewAutoGenerator()

	_ FileGenerator = DefaultAutoGenerator
)

type AutoGenerator struct{}

func NewAutoGenerator() *AutoGenerator {
	return &AutoGenerator{}
}

func (g *AutoGenerator) FromPaths(paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, errors.New("no paths provided")
	}

	if len(paths) == 1 {
		return g.fromPath(paths[0])
	}

	yamlPaths := []string{}
	jsonPaths := []string{}

	for _, path := range paths {
		if isYAMLFile(path) {
			yamlPaths = append(yamlPaths, path)
		}

		if isJSONFile(path) {
			jsonPaths = append(jsonPaths, path)
		}
	}

	var jsonSchema []byte

	readerGenErr := errors.New("no json paths provided")
	valueInferenceErr := errors.New("no yaml paths provided")

	if len(jsonPaths) > 0 {
		jsonSchema, readerGenErr = DefaultReaderGenerator.FromPaths(jsonPaths...)
		if readerGenErr == nil {
			return jsonSchema, nil
		}

		readerGenErr = fmt.Errorf("failed to read JSON Schema: %w", readerGenErr)
	}

	if len(yamlPaths) > 0 {
		jsonSchema, valueInferenceErr = DefaultValueInferenceGenerator.FromPaths(yamlPaths...)
		if valueInferenceErr == nil {
			return jsonSchema, nil
		}

		valueInferenceErr = fmt.Errorf("failed to infer JSON Schema: %w", valueInferenceErr)
	}

	return nil, fmt.Errorf("failed to generate JSON Schema: %w, %w", readerGenErr, valueInferenceErr)
}

func (g *AutoGenerator) fromPath(path string) ([]byte, error) {
	if isYAMLFile(path) {
		return DefaultValueInferenceGenerator.FromPaths(path)
	}

	if isJSONFile(path) {
		return DefaultReaderGenerator.FromPaths(path)
	}

	return nil, fmt.Errorf("unsupported file type, must be json or yaml: %s", path)
}

func (g *AutoGenerator) FromData(data []byte, refBasePath string) ([]byte, error) {
	jsonSchema, readerGenErr := DefaultReaderGenerator.FromData(data, refBasePath)
	if readerGenErr == nil {
		return jsonSchema, nil
	}

	readerGenErr = fmt.Errorf("failed to read JSON Schema: %w", readerGenErr)

	jsonSchema, valueInferenceErr := DefaultValueInferenceGenerator.FromData(data)
	if valueInferenceErr == nil {
		return jsonSchema, nil
	}

	valueInferenceErr = fmt.Errorf("failed to infer JSON Schema: %w", valueInferenceErr)

	return nil, fmt.Errorf("failed to generate JSON Schema: %w, %w", readerGenErr, valueInferenceErr)
}
