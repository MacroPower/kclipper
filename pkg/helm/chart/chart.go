package helm

import (
	"bytes"
	"fmt"

	"github.com/invopop/jsonschema"
	"kcl-lang.io/kcl-go/pkg/tools/gen"

	genutil "github.com/MacroPower/kclx/pkg/util/gen"
)

// Chart represents the KCL schema `helm.Chart`.
type Chart struct {
	Chart           string  `json:"chart" jsonschema:"description=The Helm chart name."`
	RepoURL         string  `json:"repoURL" jsonschema:"description=The URL of the Helm chart repository."`
	TargetRevision  string  `json:"targetRevision" jsonschema:"description=The semver tag for the chart's version."`
	ReleaseName     *string `json:"releaseName,omitempty" jsonschema:"-,description=The Helm release name to use. If omitted it will use the chart name."`
	SkipCRDs        *bool   `json:"skipCRDs,omitempty" jsonschema:"-,description=Skip the custom resource definition installation step (--skip-crds)."`
	PassCredentials *bool   `json:"passCredentials,omitempty" jsonschema:"-,description=Pass credentials to all domains (--pass-credentials)."`
	SchemaMode      *string `json:"schemaMode,omitempty" jsonschema:"-,description=The schema mode to use. Options are 'auto', 'values', or 'none'."`
	SchemaURL       *string `json:"schemaURL,omitempty" jsonschema:"-,description=The URL of the schema to use. If set, it will override schemaMode."`
}

func (c *Chart) GenerateKcl(b *bytes.Buffer) error {
	r := &jsonschema.Reflector{
		DoNotReference: true,
		ExpandedStruct: true,
	}
	js := r.Reflect(&Chart{})
	if cv, ok := js.Properties.Get("chart"); ok {
		cv.Default = c.Chart
	}
	if cv, ok := js.Properties.Get("repoURL"); ok {
		cv.Default = c.RepoURL
	}
	if cv, ok := js.Properties.Get("targetRevision"); ok {
		cv.Default = c.TargetRevision
	}

	jsBytes, err := js.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal json schema: %w", err)
	}

	if err := genutil.Safe.GenKcl(b, "chart", jsBytes, &gen.GenKclOptions{
		Mode:          gen.ModeJsonSchema,
		CastingOption: gen.OriginalName,
	}); err != nil {
		return fmt.Errorf("failed to generate kcl schema: %w", err)
	}

	return nil
}
