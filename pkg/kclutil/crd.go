package kclutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"kcl-lang.io/kcl-go"

	crdgen "kcl-lang.io/kcl-openapi/pkg/kube_resource/generator"
	swaggergen "kcl-lang.io/kcl-openapi/pkg/swagger/generator"

	"github.com/MacroPower/kclipper/pkg/syncutil"
)

// CRDGeneratorType defines the type of CRD generator to use.
type CRDGeneratorType string

const (
	// CRDGeneratorTypeDefault is the default generator type.
	CRDGeneratorTypeDefault CRDGeneratorType = ""
	// CRDGeneratorTypeTemplate uses a template-based generator.
	CRDGeneratorTypeTemplate CRDGeneratorType = "TEMPLATE"
	// CRDGeneratorTypeChartPath uses a chart path-based generator.
	CRDGeneratorTypeChartPath CRDGeneratorType = "CHART-PATH"
	// CRDGeneratorTypeNone skips CRD generation.
	CRDGeneratorTypeNone CRDGeneratorType = "NONE"
)

var (
	// GenOpenAPI is a concurrency-safe KCL OpenAPI/CRD generator.
	GenOpenAPI = &genOpenAPI{
		locker: syncutil.NewKeyLock(),
	}

	// CRDGeneratorTypeEnum lists all valid CRD generator types.
	CRDGeneratorTypeEnum = []any{
		CRDGeneratorTypeTemplate,
		CRDGeneratorTypeChartPath,
		CRDGeneratorTypeNone,
	}
)

// genOpenAPI is a concurrency-safe KCL OpenAPI/CRD generator implementation.
type genOpenAPI struct {
	locker *syncutil.KeyLock
}

// FromCRD generates KCL schemas from a Kubernetes CRD and saves them to the destination path.
func (g *genOpenAPI) FromCRD(crd *unstructured.Unstructured, dstPath string) error {
	opts := new(swaggergen.GenOpts)
	if err := opts.EnsureDefaults(); err != nil {
		return fmt.Errorf("failed to ensure default generator options: %w", err)
	}

	crdVersions, err := splitCRDVersions(crd)
	if err != nil {
		return fmt.Errorf("failed to split CRD versions: %w", err)
	}

	var merr error
	for version, v := range crdVersions {
		if err := g.fromCRDVersion(&v, dstPath, version, opts); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("%s: %w", v.GetAPIVersion(), err))
		}
	}
	if merr != nil {
		return multierror.Prefix(merr, ErrGenerateKCL.Error()+":") //nolint:wrapcheck // Multierror
	}

	// Format the generated KCL files.
	if _, err := kcl.FormatPath(filepath.Join(dstPath, "...")); err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func (g *genOpenAPI) fromCRDVersion(crd *unstructured.Unstructured, dstPath, version string, opts *swaggergen.GenOpts) error {
	apiVersion := crd.GetAPIVersion()
	g.locker.Lock(apiVersion)
	defer g.locker.Unlock(apiVersion)

	tmpFile, err := os.CreateTemp(os.TempDir(), "kcl-swagger-")
	if err != nil {
		return fmt.Errorf("create temp file: %w: %w", ErrWriteFile, err)
	}

	crdData, err := yaml.Marshal(crd.UnstructuredContent())
	if err != nil {
		return fmt.Errorf("marshal CRD: %w: %w", ErrYAMLMarshal, err)
	}

	if _, err := tmpFile.Write(crdData); err != nil {
		return fmt.Errorf("write CRD to temp file: %w: %w", ErrWriteFile, err)
	}

	err = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	spec, err := crdgen.GetSpec(&crdgen.GenOpts{
		Spec: tmpFile.Name(),
	})
	if err != nil {
		return fmt.Errorf("generate CRD spec: %w", err)
	}
	defer func() {
		_ = os.Remove(spec)
		_ = os.Remove(filepath.Join(filepath.Base(spec), "k8s.json"))
	}()

	opts.Spec = spec
	opts.Target = dstPath
	opts.ModelPackage = version
	opts.ValidateSpec = false

	err = swaggergen.Generate(opts)
	if err != nil {
		return fmt.Errorf("generate swagger: %w", err)
	}

	err = os.RemoveAll(filepath.Join(opts.Target, opts.ModelPackage, "k8s"))
	if err != nil {
		return fmt.Errorf("remove temp 'k8s' model package: %w", err)
	}

	return nil
}

// splitCRDVersions separates a CRD with multiple versions into individual CRDs.
func splitCRDVersions(crd *unstructured.Unstructured) (map[string]unstructured.Unstructured, error) {
	spec, ok := crd.Object["spec"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: invalid spec field", ErrInvalidFormat)
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return nil, fmt.Errorf("%w: invalid spec.versions field", ErrInvalidFormat)
	}

	crdVersions := make(map[string]unstructured.Unstructured, len(versions))
	for _, version := range versions {
		version, ok := version.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec.versions[] field", ErrInvalidFormat)
		}

		versionName, ok := version["name"].(string)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec.versions[].name field", ErrInvalidFormat)
		}

		crdVersion := crd.DeepCopy()
		versionSpec, ok := crdVersion.Object["spec"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: invalid spec field after deep copy", ErrInvalidFormat)
		}

		versionSpec["versions"] = []any{version}
		crdVersions[versionName] = *crdVersion
	}

	return crdVersions, nil
}
