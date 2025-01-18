package helm

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argohelm "github.com/MacroPower/kclipper/pkg/argoutil/helm"
	"github.com/MacroPower/kclipper/pkg/argoutil/kube"
	"github.com/MacroPower/kclipper/pkg/helmrepo"
)

type TemplateOpts struct {
	ChartName            string
	TargetRevision       string
	RepoURL              string
	ReleaseName          string
	Namespace            string
	ValuesObject         map[string]any
	SkipCRDs             bool
	KubeVersion          string
	APIVersions          []string
	PassCredentials      bool
	Proxy                string
	NoProxy              string
	SkipSchemaValidation bool
}

type ChartClient interface {
	Pull(chart, repoURL, targetRevision string, repos helmrepo.Getter) (string, error)
}

type Commander interface {
	Fetch()
	Template()
}

type JSONSchemaGenerator interface {
	FromPaths(paths ...string) ([]byte, error)
}

type Chart struct {
	Client       ChartClient
	Repos        helmrepo.Getter
	TemplateOpts TemplateOpts

	path string
}

func NewChart(client ChartClient, repos helmrepo.Getter, opts TemplateOpts) (*Chart, error) {
	chartPath, err := client.Pull(opts.ChartName, opts.RepoURL, opts.TargetRevision, repos)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %w", err)
	}

	return &Chart{
		Client:       client,
		Repos:        repos,
		TemplateOpts: opts,
		path:         chartPath,
	}, nil
}

// Template pulls a Helm chart using the provided [TemplateOpts], and then
// executes `helm template` to render the chart. The rendered output is then
// split into individual Kubernetes objects and returned as a slice of
// [unstructured.Unstructured] objects.
func (c *Chart) Template() ([]*unstructured.Unstructured, error) {
	out, err := c.template()
	if err != nil {
		return nil, err
	}

	objs, err := kube.SplitYAML(out)
	if err != nil {
		return nil, fmt.Errorf("error parsing helm template output: %w", err)
	}

	return objs, nil
}

func (c *Chart) template() ([]byte, error) {
	cmd, err := argohelm.NewCmdWithVersion(c.path, c.TemplateOpts.Proxy, c.TemplateOpts.NoProxy)
	if err != nil {
		return nil, fmt.Errorf("error creating helm command: %w", err)
	}

	out, _, err := cmd.Template(".", &argohelm.TemplateOpts{
		Name:                 c.TemplateOpts.ChartName,
		Namespace:            c.TemplateOpts.Namespace,
		Values:               c.TemplateOpts.ValuesObject,
		SkipCrds:             c.TemplateOpts.SkipCRDs,
		KubeVersion:          c.TemplateOpts.KubeVersion,
		APIVersions:          c.TemplateOpts.APIVersions,
		SkipSchemaValidation: c.TemplateOpts.SkipSchemaValidation,
		RepoGetter:           c.Repos,
		DependencyPuller:     c.Client,
	})
	if err != nil {
		return nil, fmt.Errorf("error templating helm chart: %w", err)
	}

	return []byte(out), nil
}

type ChartFileClient interface {
	PullAndExtract(chart, repoURL, targetRevision string, repos helmrepo.Getter) (string, io.Closer, error)
}

type ChartFiles struct {
	Client       ChartFileClient
	TemplateOpts TemplateOpts

	path   string
	closer io.Closer
}

func NewChartFiles(client ChartFileClient, repos helmrepo.Getter, opts TemplateOpts) (*ChartFiles, error) {
	chartPath, closer, err := client.PullAndExtract(opts.ChartName, opts.RepoURL, opts.TargetRevision, repos)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %w", err)
	}

	return &ChartFiles{
		Client:       client,
		TemplateOpts: opts,
		path:         chartPath,
		closer:       closer,
	}, nil
}

// GetValuesJSONSchema pulls a Helm chart using the provided [TemplateOpts], and
// then uses the [JSONSchemaGenerator] to generate a JSON Schema using one or
// more files from the chart. The [match] function can be used to match a subset
// of the pulled files in the chart directory for JSON Schema generation.
func (c *ChartFiles) GetValuesJSONSchema(gen JSONSchemaGenerator, match func(string) bool) ([]byte, error) {
	unmatchedFiles := []string{}
	matchedFiles := []string{}
	err := filepath.Walk(c.path,
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error walking helm chart directory: %w", err)
			}
			relPath, err := filepath.Rel(c.path, path)
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
		errMsg := "successfully pulled '%s' into '%s', but failed to find any input files" +
			"for the provided JSON Schema generator; the following paths were searched:\n%s"
		return nil, fmt.Errorf(errMsg, c.TemplateOpts.ChartName, c.path, unmatchedFileStr)
	}

	jsonSchema, err := gen.FromPaths(matchedFiles...)
	if err != nil {
		return nil, fmt.Errorf("error converting values schema to JSON Schema: %w", err)
	}

	return jsonSchema, nil
}

func (c *ChartFiles) GetCRDs(match func(string) bool) ([][]byte, error) {
	unmatchedFiles := []string{}
	matchedFiles := []string{}
	err := filepath.Walk(c.path,
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error walking helm chart directory: %w", err)
			}
			relPath, err := filepath.Rel(c.path, path)
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
		errMsg := "successfully pulled '%s' into '%s', but failed to find any CRDs; " +
			"the following paths were searched:\n%s"
		return nil, fmt.Errorf(errMsg, c.TemplateOpts.ChartName, c.path, unmatchedFileStr)
	}

	crdBytes := [][]byte{}
	for _, f := range matchedFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("error reading CRD file: %w", err)
		}
		crdBytes = append(crdBytes, b)
	}

	return crdBytes, nil
}

func (c *ChartFiles) Dispose() {
	if c.closer != nil {
		tryClose(c.closer)
	}
}
