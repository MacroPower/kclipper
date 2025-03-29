package jsonschema

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
	"kcl-lang.io/kcl-go/pkg/tools/gen"

	helmschema "github.com/dadav/helm-schema/pkg/schema"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

// Error types for the jsonschema package.
var (
	// ErrUnmarshalSchema indicates an error occurred while unmarshaling the JSON Schema.
	ErrUnmarshalSchema = errors.New("failed to unmarshal JSON Schema")

	// ErrSchemaToJSON indicates an error occurred while converting a schema to JSON.
	ErrSchemaToJSON = errors.New("failed to convert schema to JSON")

	// ErrGenerateKCL indicates an error occurred during KCL schema generation.
	ErrGenerateKCL = errors.New("failed to generate KCL schema")
)

// ConvertToKCLSchema converts a JSON schema to a KCL schema.
func ConvertToKCLSchema(jsonSchemaData []byte, removeDefaults bool) ([]byte, error) {
	fixedJSONSchema, err := ConvertToKCLCompatibleJSONSchema(jsonSchemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to KCL compatible JSON schema: %w", err)
	}

	kclSchema := &bytes.Buffer{}
	if err := kclutil.Gen.GenKcl(kclSchema, "values", fixedJSONSchema, &kclutil.GenKclOptions{
		Mode:                  gen.ModeJsonSchema,
		CastingOption:         gen.OriginalName,
		UseIntegersForNumbers: true,
		RemoveDefaults:        removeDefaults,
	}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGenerateKCL, err)
	}

	return kclSchema.Bytes(), nil
}

// ConvertToKCLCompatibleJSONSchema converts a JSON schema to a JSON schema that
// is compatible with KCL schema generation (i.e. removing unsupported fields).
func ConvertToKCLCompatibleJSONSchema(jsonSchemaData []byte) ([]byte, error) {
	// YAML is a superset of JSON, so this works and is simpler than re-writing
	// the Unmarshaler for JSON.
	var jsonNode yaml.Node
	if err := yaml.Unmarshal(jsonSchemaData, &jsonNode); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmarshalSchema, err)
	}

	hs := &helmschema.Schema{}
	if err := hs.UnmarshalYAML(&jsonNode); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmarshalSchema, err)
	}

	// Remove the ID to keep KCL schema naming consistent.
	hs.Id = ""

	// For now, merge into an empty schema as that will result in a schema that
	// is compatible with KCL schema generation.
	mhs := &helmschema.Schema{}
	mhs = mergeHelmSchemas(mhs, hs, true)

	fixedJSONSchema, err := mhs.ToJson()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaToJSON, err)
	}

	return fixedJSONSchema, nil
}
