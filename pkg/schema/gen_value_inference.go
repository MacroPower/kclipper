package schema

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"go.jacobcolvin.com/x/magicschema"
	"go.jacobcolvin.com/x/magicschema/helm"
)

// Annotator names supported by [ValueInferenceConfig.Annotators], matching the
// annotation parsers registered by [helm.DefaultRegistry].
const (
	HelmSchemaAnnotator       string = "helm-schema"
	HelmValuesSchemaAnnotator string = "helm-values-schema"
	BitnamiAnnotator          string = "bitnami"
	HelmDocsAnnotator         string = "helm-docs"
)

var (
	// ErrUnknownAnnotator indicates that an annotator name does not match any
	// registered annotation parser.
	ErrUnknownAnnotator = errors.New("unknown annotator")

	// DefaultAnnotators is the default annotator priority order used when
	// [ValueInferenceConfig.Annotators] is empty.
	DefaultAnnotators = []string{
		HelmSchemaAnnotator,
		HelmValuesSchemaAnnotator,
		BitnamiAnnotator,
		HelmDocsAnnotator,
	}

	// AnnotatorEnum lists all supported annotator names.
	AnnotatorEnum = []any{
		HelmSchemaAnnotator,
		HelmValuesSchemaAnnotator,
		BitnamiAnnotator,
		HelmDocsAnnotator,
	}

	// DefaultValueInferenceGenerator is an opinionated [ValueInferenceGenerator].
	DefaultValueInferenceGenerator = mustNewValueInferenceGenerator(&ValueInferenceConfig{
		InferDefaults: true,
	})

	// DefaultValuesFileRegex matches the chart's primary values file, whose
	// observed values become schema defaults.
	defaultValuesFileRegex = regexp.MustCompile(`^(.*/)?values\.ya?ml$`)

	_ FileGenerator = DefaultValueInferenceGenerator
)

// schemaRefComment marks values files that already reference a JSON Schema.
const schemaRefComment = `# yaml-language-server: $schema=`

// ValueInferenceConfig configures a [ValueInferenceGenerator].
type ValueInferenceConfig struct {
	// Annotation parsers to enable, in priority order.
	// Defaults to [DefaultAnnotators].
	Annotators []string `json:"annotators,omitempty"`
	// Set additionalProperties to false on objects in the generated schema.
	Strict bool `json:"strict,omitempty"`
	// Record observed YAML values as schema defaults when no annotation
	// provides one.
	InferDefaults bool `json:"inferDefaults,omitempty"`
}

// ValueInferenceGenerator is a generator that infers a JSON Schema from one or
// more Helm values files, using [magicschema].
//
// Create instances with [NewValueInferenceGenerator].
type ValueInferenceGenerator struct {
	gen *magicschema.Generator
}

// NewValueInferenceGenerator creates a new [ValueInferenceGenerator] using the
// given [ValueInferenceConfig]. It returns an error if the configuration names
// an annotator that is not registered in [helm.DefaultRegistry].
func NewValueInferenceGenerator(c *ValueInferenceConfig) (*ValueInferenceGenerator, error) {
	names := c.Annotators
	if len(names) == 0 {
		names = DefaultAnnotators
	}

	registry := helm.DefaultRegistry()
	annotators := make([]magicschema.Annotator, 0, len(names))

	for _, name := range names {
		annotator, ok := registry[name]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownAnnotator, name)
		}

		annotators = append(annotators, annotator)
	}

	opts := []magicschema.Option{
		magicschema.WithAnnotators(annotators...),
	}
	if c.Strict {
		opts = append(opts, magicschema.WithStrict(true))
	}

	if c.InferDefaults {
		opts = append(opts, magicschema.WithInferDefaults(true))
	}

	return &ValueInferenceGenerator{gen: magicschema.NewGenerator(opts...)}, nil
}

func mustNewValueInferenceGenerator(c *ValueInferenceConfig) *ValueInferenceGenerator {
	g, err := NewValueInferenceGenerator(c)
	if err != nil {
		panic(err)
	}

	return g
}

// FromPaths generates a JSON Schema from one or more file paths pointing to
// Helm values files. If multiple file paths are provided, the schemas are
// merged into a single schema with [magicschema] union semantics: properties
// are unioned, incompatible types drop the type constraint entirely, and a
// value that is null or empty in one file but typed in another widens to a
// [type, null] union so that every input stays valid against the merged
// schema. Inferred defaults follow input order (first input wins), so paths
// matching the chart's primary values file (values.yaml) are ordered first.
func (g *ValueInferenceGenerator) FromPaths(paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, errors.New("no file paths provided")
	}

	// Order the primary values file first so its observed values win merged
	// metadata like defaults and descriptions; sort within each group for
	// deterministic output.
	slices.SortFunc(paths, func(a, b string) int {
		if pa, pb := isPrimaryValuesFile(a), isPrimaryValuesFile(b); pa != pb {
			if pa {
				return -1
			}

			return 1
		}

		return strings.Compare(a, b)
	})

	inputs := make([][]byte, 0, len(paths))

	for _, path := range paths {
		//nolint:gosec // G304 not relevant for client-side generation.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read values file %q: %w", path, err)
		}

		inputs = append(inputs, data)
	}

	return g.generate(inputs...)
}

// FromData generates a JSON Schema from the given values file content.
func (g *ValueInferenceGenerator) FromData(data []byte) ([]byte, error) {
	return g.generate(data)
}

// isPrimaryValuesFile reports whether path is the chart's primary values file.
func isPrimaryValuesFile(path string) bool {
	return defaultValuesFileRegex.MatchString(path)
}

func (g *ValueInferenceGenerator) generate(inputs ...[]byte) ([]byte, error) {
	for _, input := range inputs {
		// Check if a schema reference exists in the values file.
		if bytes.Contains(input, []byte(schemaRefComment)) {
			return nil, errors.New("schema reference already exists in values file")
		}
	}

	schema, err := g.gen.Generate(inputs...)
	if err != nil {
		return nil, fmt.Errorf("infer schema from values: %w", err)
	}

	return marshalSchema(schema)
}
