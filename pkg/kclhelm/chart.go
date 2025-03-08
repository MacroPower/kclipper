package kclhelm

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
)

// Defines a Helm chart.
type Chart struct {
	// Lambda function to modify the Helm template output. Evaluated for each resource in the Helm template output.
	PostRenderer any `json:"postRenderer,omitempty"`
	// Helm value files to be passed to Helm template.
	ValueFiles []string `json:"valueFiles,omitempty"`
}

func (c *Chart) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}

	js := r.Reflect(reflect.TypeOf(Chart{}))

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b, genOptInheritChartBase, genOptFixChartRepo, genOptFixPostRenderer)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	if _, err := b.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}
