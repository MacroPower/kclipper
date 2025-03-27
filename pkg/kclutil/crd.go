package kclutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"kcl-lang.io/kcl-go"

	crdgen "kcl-lang.io/kcl-openapi/pkg/kube_resource/generator"
	swaggergen "kcl-lang.io/kcl-openapi/pkg/swagger/generator"
)

type CRDGeneratorType string

const (
	CRDGeneratorTypeDefault   CRDGeneratorType = ""
	CRDGeneratorTypeTemplate  CRDGeneratorType = "TEMPLATE"
	CRDGeneratorTypeChartPath CRDGeneratorType = "CHART-PATH"
	CRDGeneratorTypeNone      CRDGeneratorType = "NONE"
)

var (
	// GenCRD is a concurrency-safe KCL generator.
	GenOpenAPI = &genOpenAPI{}

	CRDGeneratorTypeEnum = []any{
		CRDGeneratorTypeTemplate,
		CRDGeneratorTypeChartPath,
		CRDGeneratorTypeNone,
	}
)

type genOpenAPI struct {
	mu sync.Mutex
}

func (g *genOpenAPI) FromCRD(crd *unstructured.Unstructured, dstPath string) error {
	opts := new(swaggergen.GenOpts)
	if err := opts.EnsureDefaults(); err != nil {
		return fmt.Errorf("failed to ensure default generator options: %w", err)
	}

	crdVersions, err := splitCRDVersions(crd)
	if err != nil {
		return fmt.Errorf("failed to split CRD versions: %w", err)
	}

	for version, v := range crdVersions {
		if err := g.fromCRDVersion(&v, dstPath, version, opts); err != nil {
			return fmt.Errorf("failed to generate KCL Schemas for %s: %w", version, err)
		}
	}

	return nil
}

func (g *genOpenAPI) fromCRDVersion(crd *unstructured.Unstructured, dstPath, version string, opts *swaggergen.GenOpts) error {
	tmpFile, err := os.CreateTemp(os.TempDir(), "kcl-swagger-")
	if err != nil {
		return fmt.Errorf("failed to create temp file for spec: %w", err)
	}

	crdData, err := yaml.Marshal(crd.UnstructuredContent())
	if err != nil {
		return fmt.Errorf("failed to marshal CRD: %w", err)
	}

	if _, err := tmpFile.Write(crdData); err != nil {
		return fmt.Errorf("failed to write CRD to temp file: %w", err)
	}

	err = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	spec, err := crdgen.GetSpec(&crdgen.GenOpts{
		Spec: tmpFile.Name(),
	})
	if err != nil {
		return fmt.Errorf("failed to generate CRD spec: %w", err)
	}
	defer func() {
		_ = os.Remove(spec)
		_ = os.Remove(filepath.Join(filepath.Base(spec), "k8s.json"))
	}()

	opts.Spec = spec
	opts.Target = dstPath
	opts.ModelPackage = version
	opts.ValidateSpec = false

	g.mu.Lock()
	err = swaggergen.Generate(opts)
	g.mu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to generate KCL Schema: %w", err)
	}

	err = os.RemoveAll(filepath.Join(opts.Target, opts.ModelPackage, "k8s"))
	if err != nil {
		return fmt.Errorf("failed to remove temp 'k8s' model package: %w", err)
	}

	// Format the generated KCL files.
	if _, err := kcl.FormatPath(filepath.Join(dstPath, version)); err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func splitCRDVersions(crd *unstructured.Unstructured) (map[string]unstructured.Unstructured, error) {
	spec, ok := crd.Object["spec"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid spec field")
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return nil, errors.New("invalid spec.versions field")
	}

	crdVersions := make(map[string]unstructured.Unstructured, len(versions))

	for _, version := range versions {
		version, ok := version.(map[string]any)
		if !ok {
			return nil, errors.New("invalid spec.versions[] field")
		}

		versionName, ok := version["name"].(string)
		if !ok {
			return nil, errors.New("invalid spec.versions[].name field")
		}

		crdVersion := crd.DeepCopy()

		versionSpec, ok := crdVersion.Object["spec"].(map[string]any)
		if !ok {
			return nil, errors.New("invalid spec field after deep copy")
		}
		versionSpec["versions"] = []any{version}

		crdVersions[versionName] = *crdVersion
	}

	return crdVersions, nil
}
