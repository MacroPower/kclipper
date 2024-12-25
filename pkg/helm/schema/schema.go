// Copyright (c) 2023 dadav. MIT License

package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	helmschema "github.com/dadav/helm-schema/pkg/schema"
	"github.com/dadav/helm-schema/pkg/util"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

var DefaultGenerator = NewGenerator()

type Generator struct {
	skipAutoGenerationConfig *helmschema.SkipAutoGenerationConfig

	uncomment                 bool
	keepFullComment           bool
	helmDocsCompatibilityMode bool
	dontRemoveHelmDocsPrefix  bool
}

func NewGenerator() *Generator {
	return &Generator{
		skipAutoGenerationConfig: &helmschema.SkipAutoGenerationConfig{
			Required:             true,
			AdditionalProperties: true,
		},
	}
}

func (s *Generator) Create(valuesPath string) (*helmschema.Schema, error) {
	valuesFile, err := os.Open(valuesPath)
	if err != nil {
		return nil, fmt.Errorf("error opening values file: %w", err)
	}
	content, err := util.ReadFileAndFixNewline(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("error reading values file: %w", err)
	}

	// Check if a schema reference exists in the yaml file
	schemaRef := `# yaml-language-server: $schema=`
	if strings.Contains(string(content), schemaRef) {
		return nil, errors.New("schema reference already exists in values file")
	}

	// Optional preprocessing
	if s.uncomment {
		// Remove comments from valid yaml
		content, err = util.RemoveCommentsFromYaml(bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("error uncommenting yaml: %w", err)
		}
	}

	var values yaml.Node
	err = yaml.Unmarshal(content, &values)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling values yaml: %w", err)
	}

	valuesSchema := helmschema.YamlToSchema(valuesPath, &values, s.keepFullComment,
		s.helmDocsCompatibilityMode, s.dontRemoveHelmDocsPrefix, s.skipAutoGenerationConfig, nil)

	err = s.FixSchema(valuesSchema)
	if err != nil {
		return nil, fmt.Errorf("error fixing schema: %w", err)
	}

	return valuesSchema, nil
}

func (s *Generator) FixSchema(hs *helmschema.Schema) error {
	if err := FixSchema(hs, AllowAdditionalProperties); err != nil {
		return err
	}

	return nil
}

func (s *Generator) GetJSONSchema(valuesPath string) (*jsonschema.Schema, error) {
	js := jsonschema.NewCompiler()

	valuesSchema, err := s.Create(valuesPath)
	if err != nil {
		return nil, err
	}

	valuesSchemaJSONBytes, err := valuesSchema.ToJson()
	if err != nil {
		return nil, fmt.Errorf("error converting values schema to JSON: %w", err)
	}

	valuesJSONSchema, err := js.Compile(string(valuesSchemaJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("error compiling json schema: %w", err)
	}

	return valuesJSONSchema, nil
}

func UnmarshalJSON(data []byte) (*helmschema.Schema, error) {
	c := jsonschema.NewCompiler()
	if err := c.AddResource("values.schema.json", bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("invalid schema syntax: %w", err)
	}
	hs := &helmschema.Schema{}
	if err := json.Unmarshal(data, hs); err != nil {
		return nil, fmt.Errorf("error unmarshaling schema: %w", err)
	}
	if err := hs.Validate(); err != nil {
		return nil, fmt.Errorf("error validating schema: %w", err)
	}
	if err := helmschema.FixRequiredProperties(hs); err != nil {
		return nil, fmt.Errorf("error fixing required properties: %w", err)
	}
	if err := FixSchema(hs, AllowAdditionalProperties); err != nil {
		return nil, fmt.Errorf("error fixing additionalProperties: %w", err)
	}
	return hs, nil
}

func AllowAdditionalProperties(s *helmschema.Schema) error {
	if s.Type.Matches("object") {
		s.AdditionalProperties = true
	}

	return nil
}

func FixSchema(s *helmschema.Schema, fn func(s *helmschema.Schema) error) error {
	if err := fn(s); err != nil {
		return err
	}
	for _, v := range s.Properties {
		if err := fn(v); err != nil {
			return err
		}
	}
	if s.Items != nil {
		if err := fn(s.Items); err != nil {
			return err
		}
	}

	if s.AnyOf != nil {
		for _, v := range s.AnyOf {
			if err := fn(v); err != nil {
				return err
			}
		}
	}
	if s.OneOf != nil {
		for _, v := range s.OneOf {
			if err := fn(v); err != nil {
				return err
			}
		}
	}
	if s.AllOf != nil {
		for _, v := range s.AllOf {
			if err := fn(v); err != nil {
				return err
			}
		}
	}
	if s.If != nil {
		if err := fn(s.If); err != nil {
			return err
		}
	}
	if s.Else != nil {
		if err := fn(s.Else); err != nil {
			return err
		}
	}
	if s.Then != nil {
		if err := fn(s.Then); err != nil {
			return err
		}
	}
	if s.Not != nil {
		if err := fn(s.Not); err != nil {
			return err
		}
	}
	return nil
}
