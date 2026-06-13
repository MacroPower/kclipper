package kclhelm

import (
	"bytes"
	"fmt"
	"io"

	"github.com/macropower/kclipper/pkg/crd"
	"github.com/macropower/kclipper/pkg/schema"
)

// Configuration that can be defined in `charts.k`, in addition to those
// specified in `helm.ChartBase`.
type ChartConfig struct {
	// Schema generator to use for the Values schema.
	SchemaGenerator schema.GeneratorType `json:"schemaGenerator,omitempty"`
	// Path to the schema to use, when relevant for the selected schemaGenerator.
	SchemaPath string `json:"schemaPath,omitempty"`
	// Configuration for value inference via magicschema. Requires
	// schemaGenerator to be set to `VALUE-INFERENCE`.
	ValueInference *ValueInferenceConfig `json:"valueInference,omitempty"`
	// CRD generator to use for CRDs schemas.
	CRDGenerator crd.GeneratorType `json:"crdGenerator,omitempty"`
	// Paths to any CRDs to import as schemas, when relevant for the selected
	// crdGenerator. Can be file and/or URL paths. Glob patterns are supported.
	CRDPaths []string `json:"crdPaths,omitempty"`
}

func (c *ChartConfig) GenerateKCL(w io.Writer) error {
	js, err := schema.Reflect[ChartConfig](schema.WithGoComments())
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	js.SetProperty("schemaGenerator", schema.WithEnum(schema.GeneratorTypeEnum))
	js.SetProperty("crdGenerator", schema.WithEnum(crd.GeneratorTypeEnum))
	js.SetProperty("valueInference", schema.WithType("null"), schema.WithNoContent())

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b, genOptInheritChartBase, genOptFixValueInference)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	_, err = b.WriteTo(w)
	if err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}
