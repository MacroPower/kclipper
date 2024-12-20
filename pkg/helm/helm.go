package helm

import (
	"fmt"
	"io"
	"os"
	"path"

	helmutil "github.com/argoproj/argo-cd/v2/util/helm"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"
	pathutil "github.com/argoproj/argo-cd/v2/util/io/path"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
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
		return "", closer, errors.Wrap(err, "error extracting helm chart")
	}

	return chartPath, closer, nil
}

func (h *Helm) writeValues(values map[string]any) (string, error) {
	valuesYAML, err := yaml.Marshal(values)
	if err != nil {
		return "", errors.Wrap(err, "error marshaling values_object to YAML")
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

func (h *Helm) Template(opts *TemplateOpts) ([]*unstructured.Unstructured, error) {
	chartPath, closer, err := h.pull(opts)
	if err != nil {
		return nil, err
	}
	defer ioutil.Close(closer)

	ha, err := helmutil.NewHelmApp(chartPath, opts.Repositories, false, opts.HelmVersion, "", "", opts.PassCredentials)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing helm app object")
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
			return nil, errors.Wrap(err, "error templating helm chart")
		}
		if err = ha.DependencyBuild(); err != nil {
			return nil, errors.Wrap(err, "error building helm dependencies")
		}
		out, _, err = ha.Template(templateOpts)
		if err != nil {
			return nil, errors.Wrap(err, "error templating helm chart")
		}
	}

	objs, err := SplitYAML([]byte(out))
	if err != nil {
		return nil, errors.Wrap(err, "error parsing helm template output")
	}

	return objs, nil
}
