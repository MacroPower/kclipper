package helmutil

import (
	"fmt"

	crdGen "kcl-lang.io/kcl-openapi/pkg/kube_resource/generator"
	swaggerGen "kcl-lang.io/kcl-openapi/pkg/swagger/generator"
)

func CRDToKCL(srcPath, dstPath string) error {
	opts := new(swaggerGen.GenOpts)
	if err := opts.EnsureDefaults(); err != nil {
		return fmt.Errorf("failed to ensure default generator options: %w", err)
	}

	spec, err := crdGen.GetSpec(&crdGen.GenOpts{
		Spec: srcPath,
	})
	if err != nil {
		return fmt.Errorf("failed to generate CRD spec: %w", err)
	}
	opts.Spec = spec
	opts.Target = dstPath
	opts.ModelPackage = "crds"
	opts.ValidateSpec = false

	err = swaggerGen.Generate(opts)
	if err != nil {
		return fmt.Errorf("failed to generate KCL Schema: %w", err)
	}

	return nil
}
