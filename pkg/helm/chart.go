package helm

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	argohelm "github.com/argoproj/argo-cd/v2/util/helm"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"
	pathutil "github.com/argoproj/argo-cd/v2/util/io/path"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type Chart struct {
	Client       ChartClient
	TemplateOpts TemplateOpts
}

type TemplateOpts struct {
	ChartName       string
	TargetRevision  string
	RepoURL         string
	ReleaseName     string
	Namespace       string
	HelmVersion     string
	ValuesObject    map[string]any
	Repositories    []argohelm.HelmRepository
	Credentials     Creds
	SkipCRDs        bool
	KubeVersion     string
	APIVersions     []string
	PassCredentials bool
	Proxy           string
	NoProxy         string
}

type ChartClient interface {
	PullWithCreds(chart, repoURL, targetRevision string, creds Creds, passCredentials bool) (string, io.Closer, error)
}

type JSONSchemaGenerator interface {
	FromPaths(paths ...string) ([]byte, error)
}

func NewChart(client ChartClient, opts TemplateOpts) *Chart {
	return &Chart{
		Client:       client,
		TemplateOpts: opts,
	}
}

// GetValuesJSONSchema pulls a Helm chart using the provided [TemplateOpts], and
// then uses the [JSONSchemaGenerator] to generate a JSON Schema using one or
// more files from the chart. The [match] function can be used to match a subset
// of the pulled files in the chart directory for JSON Schema generation.
func (h *Chart) GetValuesJSONSchema(gen JSONSchemaGenerator, match func(string) bool) ([]byte, error) {
	chartPath, closer, err := h.Client.PullWithCreds(h.TemplateOpts.ChartName, h.TemplateOpts.RepoURL,
		h.TemplateOpts.TargetRevision, h.TemplateOpts.Credentials, h.TemplateOpts.PassCredentials)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %w", err)
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
		return nil, fmt.Errorf(errMsg, h.TemplateOpts.ChartName, unmatchedFileStr)
	}

	jsonSchema, err := gen.FromPaths(matchedFiles...)
	if err != nil {
		return nil, fmt.Errorf("error converting values schema to JSON Schema: %w", err)
	}

	return jsonSchema, nil
}

func (h *Chart) Template() ([]*unstructured.Unstructured, error) {
	chartPath, closer, err := h.Client.PullWithCreds(h.TemplateOpts.ChartName, h.TemplateOpts.RepoURL,
		h.TemplateOpts.TargetRevision, h.TemplateOpts.Credentials, h.TemplateOpts.PassCredentials)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %w", err)
	}
	defer ioutil.Close(closer)

	isLocal := false
	ha, err := argohelm.NewHelmApp(chartPath, h.TemplateOpts.Repositories, isLocal, h.TemplateOpts.HelmVersion,
		h.TemplateOpts.Proxy, h.TemplateOpts.NoProxy, h.TemplateOpts.PassCredentials)
	if err != nil {
		return nil, fmt.Errorf("error initializing helm app object: %w", err)
	}
	defer ha.Dispose()

	p, err := h.writeValues(h.TemplateOpts.ValuesObject)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(p)
	}()

	argoTemplateOpts := &argohelm.TemplateOpts{
		Name:        h.TemplateOpts.ReleaseName,
		Namespace:   h.TemplateOpts.Namespace,
		ExtraValues: pathutil.ResolvedFilePath(p),
		SkipCrds:    h.TemplateOpts.SkipCRDs,
	}
	out, _, err := ha.Template(argoTemplateOpts)
	if err != nil {
		if !argohelm.IsMissingDependencyErr(err) {
			return nil, fmt.Errorf("error templating helm chart: %w", err)
		}
		if err = ha.DependencyBuild(); err != nil {
			return nil, fmt.Errorf("error building helm dependencies: %w", err)
		}
		out, _, err = ha.Template(argoTemplateOpts)
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

func (h *Chart) writeValues(values map[string]any) (string, error) {
	valuesYAML, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("error marshaling values to YAML: %w", err)
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
