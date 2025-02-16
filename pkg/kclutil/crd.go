package kclutil

import (
	"fmt"
	"os"
	"path/filepath"

	crdgen "kcl-lang.io/kcl-openapi/pkg/kube_resource/generator"
	swaggergen "kcl-lang.io/kcl-openapi/pkg/swagger/generator"
)

func GenerateKCLFromCRD(crd []byte, dstPath string) error {
	opts := new(swaggergen.GenOpts)
	if err := opts.EnsureDefaults(); err != nil {
		return fmt.Errorf("failed to ensure default generator options: %w", err)
	}

	tmpSpecDir := os.TempDir()

	tmpFile, err := os.CreateTemp(tmpSpecDir, "kcl-swagger-")
	if err != nil {
		return fmt.Errorf("failed to create temp file for spec: %w", err)
	}

	if _, err := tmpFile.Write(crd); err != nil {
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
	opts.ModelPackage = "crds"
	opts.ValidateSpec = false

	err = swaggergen.Generate(opts)
	if err != nil {
		return fmt.Errorf("failed to generate KCL Schema: %w", err)
	}

	err = os.RemoveAll(filepath.Join(opts.Target, opts.ModelPackage, "k8s"))
	if err != nil {
		return fmt.Errorf("failed to remove temp 'k8s' model package: %w", err)
	}

	return nil
}
