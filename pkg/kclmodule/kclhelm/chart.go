package kclhelm

import (
	"bytes"
	"fmt"
	"io"

	"github.com/macropower/kclipper/pkg/jsonschema"
)

// Defines a Helm chart.
type Chart struct {
	// Lambda function to modify the Helm template output. Evaluated for each resource in the Helm template output.
	PostRenderer any `json:"postRenderer,omitempty"`
	// Helm value files to be passed to Helm template.
	ValueFiles []string `json:"valueFiles,omitempty"`
}

func (c *Chart) GenerateKCL(w io.Writer) error {
	js, err := jsonschema.Reflect[Chart](jsonschema.WithGoComments())
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b, genOptInheritChartBase, genOptFixChartRepo, genOptFixPostRenderer)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	_, err = b.WriteTo(w)
	if err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}
