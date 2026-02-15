package jsonschema

import (
	"bytes"
	"fmt"

	helmschema "github.com/dadav/helm-schema/pkg/schema"

	"github.com/macropower/kclipper/pkg/kclgen"
)

// ConvertToKCLSchema converts a JSON schema to a KCL schema.
func ConvertToKCLSchema(jsonSchemaData []byte, removeDefaults bool) ([]byte, error) {
	fixedJSONSchema, err := ConvertToKCLCompatibleJSONSchema(jsonSchemaData)
	if err != nil {
		return nil, fmt.Errorf("convert to KCL compatible JSON schema: %w", err)
	}

	kclSchema := &bytes.Buffer{}
	err = kclgen.Gen.GenKcl(kclSchema, "values", fixedJSONSchema, &kclgen.GenKclOptions{
		Mode:                  kclgen.ModeJSONSchema,
		CastingOption:         kclgen.OriginalName,
		UseIntegersForNumbers: true,
		RemoveDefaults:        removeDefaults,
	})
	if err != nil {
		return nil, fmt.Errorf("generate kcl schema: %w", err)
	}

	return kclSchema.Bytes(), nil
}

// ConvertToKCLCompatibleJSONSchema converts a JSON schema to a JSON schema that
// is compatible with KCL schema generation (i.e. removing unsupported fields).
func ConvertToKCLCompatibleJSONSchema(jsonSchemaData []byte) ([]byte, error) {
	hs, err := unmarshalHelmSchema(jsonSchemaData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON Schema: %w", err)
	}

	// Remove the ID to keep KCL schema naming consistent.
	hs.Id = ""

	// For now, merge into an empty schema as that will result in a schema that
	// is compatible with KCL schema generation.
	mhs := &helmschema.Schema{}
	mhs = mergeHelmSchemas(mhs, hs, true)

	fixedJSONSchema, err := mhs.ToJson()
	if err != nil {
		return nil, fmt.Errorf("convert schema to JSON: %w", err)
	}

	return fixedJSONSchema, nil
}
