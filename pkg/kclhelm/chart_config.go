package kclhelm

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

// Configuration that can be defined in `charts.k`, in addition to those
// specified in `helm.ChartBase`.
type ChartConfig struct {
	// Schema generator to use for the Values schema.
	SchemaGenerator jsonschema.GeneratorType `json:"schemaGenerator,omitempty"`
	// Path to the schema to use, when relevant for the selected schemaGenerator.
	SchemaPath string `json:"schemaPath,omitempty"`
	// CRD generator to use for CRDs schemas.
	CRDGenerator kclutil.CRDGeneratorType `json:"crdGenerator,omitempty"`
	// Path to any CRDs to import as schemas, when relevant for the selected
	// crdGenerator. Glob patterns are supported.
	CRDPath string `json:"crdPath,omitempty"`
}

func (c *ChartConfig) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}

	js := r.Reflect(reflect.TypeOf(ChartConfig{}))

	js.SetProperty("schemaGenerator", jsonschema.WithEnum(jsonschema.GeneratorTypeEnum))
	js.SetProperty("crdGenerator", jsonschema.WithEnum(kclutil.CRDGeneratorTypeEnum))

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b, genOptInheritChartBase)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	if _, err := b.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}
