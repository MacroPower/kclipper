package helm

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"

	helmutil "github.com/argoproj/argo-cd/v2/util/helm"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"
	pathutil "github.com/argoproj/argo-cd/v2/util/io/path"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/MacroPower/kclx/pkg/jsonschema"
)

var DefaultHelm = NewHelm("10M")

var DefaultValuesFileRegex = regexp.MustCompile(`^values\.ya?ml$`)

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

// GetValuesJSONSchema pulls a Helm chart using the provided [TemplateOpts], and
// then uses the [jsonschema.FileGenerator] to generate a JSON Schema using one or
// more files from the chart. The [match] function can be used to match a subset
// of the pulled files in the chart directory for JSON Schema generation.
func (h *Helm) GetValuesJSONSchema(opts *TemplateOpts, gen jsonschema.FileGenerator, match func(string) bool) ([]byte, error) {
	chartPath, closer, err := h.pull(opts)
	if err != nil {
		return nil, err
	}
	defer ioutil.Close(closer)

	unmatchedFiles := []string{}
	matchedFiles := []string{}
	err = filepath.Walk(chartPath,
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error walking helm chart directory: %w", err)
			}
			relPath, err := filepath.Rel(chartPath, path)
			if err != nil {
				return fmt.Errorf("error getting relative path: %w", err)
			}
			// Use the relative path to match against the provided filter.
			if match(relPath) {
				// Append the unmodified/absolute path to the matched files.
				matchedFiles = append(matchedFiles, path)
			} else {
				// Append the relative path to the unmatched files, for use in error messages.
				unmatchedFiles = append(unmatchedFiles, relPath)
			}
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("error reading helm chart directory: %w", err)
	}

	if len(matchedFiles) == 0 {
		unmatchedFileStr := []string{}
		for _, f := range unmatchedFiles {
			unmatchedFileStr = append(unmatchedFileStr, fmt.Sprintf("\t%s\n", f))
		}
		errMsg := "successfully pulled '%s', but failed to find any input files for the provided JSON Schema generator; " +
			"the following paths were searched:\n%s"
		return nil, fmt.Errorf(errMsg, opts.ChartName, unmatchedFileStr)
	}

	jsonSchema, err := gen.FromPaths(matchedFiles...)
	if err != nil {
		return nil, fmt.Errorf("error converting values schema to JSON Schema: %w", err)
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
