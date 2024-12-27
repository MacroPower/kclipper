package helm

import (
	"fmt"
	"io"
	"os"
	"path"
	"regexp"

	helmutil "github.com/argoproj/argo-cd/v2/util/helm"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"
	pathutil "github.com/argoproj/argo-cd/v2/util/io/path"
	helmschema "github.com/dadav/helm-schema/pkg/schema"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/MacroPower/kclx/pkg/helm/schemagen/valuesgen"
)

var DefaultHelm = NewHelm("10M")

type Helm struct {
	chartPaths               ioutil.TempPaths
	manifestMaxExtractedSize resource.Quantity
}

func NewHelm(manifestMaxExtractedSize string) *Helm {
	chartPaths := NewTempPaths(os.TempDir())
	maxSize, err := resource.ParseQuantity(manifestMaxExtractedSize)
	if err != nil {
		panic(err)
	}

	return &Helm{
		chartPaths:               chartPaths,
		manifestMaxExtractedSize: maxSize,
	}
}

type TemplateOpts struct {
	ChartName       string
	TargetRevision  string
	RepoURL         string
	ReleaseName     string
	Namespace       string
	Project         string
	HelmVersion     string
	ValuesObject    map[string]any
	Repositories    []helmutil.HelmRepository
	Credentials     helmutil.Creds
	EnableOCI       bool
	SkipCRDs        bool
	PassCredentials bool
	KubeVersion     string
	APIVersions     []string
}

// pull will retrieve the chart from RepoURL, extract it, and return the path to
// the extracted chart. The closer will clean up the extracted chart. Pulled
// charts will be stored in .tar.gz format, and subsequent requests for the same
// combination of ChartName, TargetRevision, and Project will extract said chart
// from the .tar.gz file rather than pulling it again.
func (h *Helm) pull(opts *TemplateOpts) (string, io.Closer, error) {
	hcl := helmutil.NewClient(opts.RepoURL, opts.Credentials, opts.EnableOCI, "", "",
		helmutil.WithChartPaths(h.chartPaths))

	chartPath, closer, err := hcl.ExtractChart(opts.ChartName, opts.TargetRevision, opts.Project, opts.PassCredentials,
		h.manifestMaxExtractedSize.Value(), h.manifestMaxExtractedSize.IsZero())
	if err != nil {
		return "", closer, fmt.Errorf("error extracting helm chart: %w", err)
	}

	return chartPath, closer, nil
}

func (h *Helm) writeValues(values map[string]any) (string, error) {
	valuesYAML, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("error marshaling values_object to YAML: %w", err)
	}
	rand, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("error generating random filename for Helm values file: %w", err)
	}
	p := path.Join(os.TempDir(), rand.String())
	err = os.WriteFile(p, valuesYAML, 0o600)
	if err != nil {
		return "", fmt.Errorf("error writing helm values file: %w", err)
	}
	return p, nil
}

func (h *Helm) GetValuesJSONSchema(opts *TemplateOpts, findExistingSchemas bool) ([]byte, error) {
	valuesSchemas := map[string]helmschema.Schema{}

	chartPath, closer, err := h.pull(opts)
	if err != nil {
		return nil, err
	}
	defer ioutil.Close(closer)

	// Read all yaml files in the chart directory
	chartFiles, err := os.ReadDir(chartPath)
	if err != nil {
		return nil, fmt.Errorf("error reading helm chart directory: %w", err)
	}

	valuesRegex := regexp.MustCompile(`values.*\.ya?ml$`)
	for _, f := range chartFiles {
		if f.IsDir() {
			continue
		}
		if findExistingSchemas && f.Name() == "values.schema.json" {
			existingValuesSchema, err := os.ReadFile(path.Join(chartPath, f.Name()))
			if err != nil {
				return nil, fmt.Errorf("error reading existing values schema: %w", err)
			}
			es, err := valuesgen.UnmarshalJSON(existingValuesSchema)
			if err != nil {
				return nil, fmt.Errorf("error unmarshaling existing values schema: %w", err)
			}
			esjs, err := es.ToJson()
			if err != nil {
				return nil, fmt.Errorf("error converting existing values schema to JSON: %w", err)
			}
			return esjs, nil
		}
		if valuesRegex.MatchString(f.Name()) {
			vs, err := valuesgen.DefaultGenerator.Create(path.Join(chartPath, f.Name()))
			if err != nil {
				return nil, fmt.Errorf("error getting schema for helm values file: %w", err)
			}
			valuesSchemas[f.Name()] = *vs
		}
	}

	reDefaultValues := regexp.MustCompile(`^values\.ya?ml$`)
	mergedValueSchema := &helmschema.Schema{}
	for k, vs := range valuesSchemas {
		mergedValueSchema = valuesgen.Merge(mergedValueSchema, &vs, reDefaultValues.MatchString(k))
	}

	if err := mergedValueSchema.Validate(); err != nil {
		return nil, fmt.Errorf("error validating values schema: %w", err)
	}

	jsonSchema, err := mergedValueSchema.ToJson()
	if err != nil {
		return nil, fmt.Errorf("error converting values schema to JSON schema: %w", err)
	}

	return jsonSchema, nil
}

func (h *Helm) Template(opts *TemplateOpts) ([]*unstructured.Unstructured, error) {
	chartPath, closer, err := h.pull(opts)
	if err != nil {
		return nil, err
	}
	defer ioutil.Close(closer)

	ha, err := helmutil.NewHelmApp(chartPath, opts.Repositories, false, opts.HelmVersion, "", "", opts.PassCredentials)
	if err != nil {
		return nil, fmt.Errorf("error initializing helm app object: %w", err)
	}
	defer ha.Dispose()

	p, err := h.writeValues(opts.ValuesObject)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(p)
	}()

	templateOpts := &helmutil.TemplateOpts{
		Name:        opts.ReleaseName,
		Namespace:   opts.Namespace,
		ExtraValues: pathutil.ResolvedFilePath(p),
		SkipCrds:    opts.SkipCRDs,
	}
	out, _, err := ha.Template(templateOpts)
	if err != nil {
		if !helmutil.IsMissingDependencyErr(err) {
			return nil, fmt.Errorf("error templating helm chart: %w", err)
		}
		if err = ha.DependencyBuild(); err != nil {
			return nil, fmt.Errorf("error building helm dependencies: %w", err)
		}
		out, _, err = ha.Template(templateOpts)
		if err != nil {
			return nil, fmt.Errorf("error templating helm chart: %w", err)
		}
	}

	objs, err := SplitYAML([]byte(out))
	if err != nil {
		return nil, fmt.Errorf("error parsing helm template output: %w", err)
	}

	return objs, nil
}
