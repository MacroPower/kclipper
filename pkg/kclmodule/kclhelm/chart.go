package kclhelm

import (
	"fmt"
	"io"

	"github.com/macropower/kclipper/pkg/schema"
)

// Defines a Helm chart.
type Chart struct {
	// Lambda function to modify the Helm template output. Evaluated for each resource in the Helm template output.
	PostRenderer any `json:"postRenderer,omitempty"`
	// Helm value files to be passed to Helm template.
	ValueFiles []string `json:"valueFiles,omitempty"`
}

func (c *Chart) GenerateKCL(w io.Writer) error {
	js, err := schema.Reflect[Chart](schema.WithGoComments())
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	err = js.GenerateKCL(w, genOptInheritChartBase, genOptFixChartRepo, genOptFixPostRenderer)
	if err != nil {
		return fmt.Errorf("convert JSON Schema to KCL schema: %w", err)
	}

	return nil
}
