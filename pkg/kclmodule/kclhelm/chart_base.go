package kclhelm

import (
	"fmt"
	"io"

	"github.com/macropower/kclipper/pkg/schema"
)

// Represents attributes common in `helm.Chart` and `helm.ChartConfig`.
type ChartBase struct {
	// Helm values to be passed to Helm template. These take precedence over valueFiles.
	Values any `json:"values,omitempty"`
	// Helm chart name.
	Chart string `json:"chart"`
	// URL of the Helm chart repository.
	RepoURL string `json:"repoURL"`
	// Semver tag for the chart's version. May be omitted for local charts.
	TargetRevision string `json:"targetRevision,omitempty"`
	// Helm release name to use. If omitted the chart name will be used.
	ReleaseName string `json:"releaseName,omitempty"`
	// Optional namespace to template with.
	Namespace string `json:"namespace,omitempty"`
	// Validator to use for the Values schema.
	SchemaValidator schema.ValidatorType `json:"schemaValidator,omitempty"`
	// Helm chart repositories.
	Repositories []ChartRepo `json:"repositories,omitempty"`
	// Set to `True` to skip the custom resource definition installation step (Helm's `--skip-crds`).
	SkipCRDs bool `json:"skipCRDs,omitempty"`
	// Set to `True` to skip templating Helm hooks (similar to Helm's `--no-hooks`).
	SkipHooks bool `json:"skipHooks,omitempty"`
	// Set to `True` to pass credentials to all domains (Helm's `--pass-credentials`).
	PassCredentials bool `json:"passCredentials,omitempty"`
}

func (c *ChartBase) GenerateKCL(w io.Writer) error {
	js, err := schema.Reflect[ChartBase](schema.WithGoComments())
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	js.SetProperty("schemaValidator", schema.WithEnum(schema.ValidatorTypeEnum))
	js.SetProperty("repositories", schema.WithType("null"), schema.WithNoContent())

	err = js.GenerateKCL(w, genOptFixChartRepo)
	if err != nil {
		return fmt.Errorf("convert JSON Schema to KCL schema: %w", err)
	}

	return nil
}
