package kclgen

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	crdgen "kcl-lang.io/kcl-openapi/pkg/kube_resource/generator"
	swaggergen "kcl-lang.io/kcl-openapi/pkg/swagger/generator"

	"github.com/MacroPower/kclipper/pkg/kclerrors"
	"github.com/MacroPower/kclipper/pkg/syncs"
)

// GenOpenAPI is a concurrency-safe KCL OpenAPI/CRD generator.
var GenOpenAPI = mustNewGenOpenAPI()

// genOpenAPI is a concurrency-safe KCL OpenAPI/CRD generator implementation.
type genOpenAPI struct {
	locker *syncs.KeyLock
}

func mustNewGenOpenAPI() *genOpenAPI {
	return &genOpenAPI{
		locker: syncs.NewKeyLock(),
	}
}

// FromCRDVersion generates a new KCL module from a Kubernetes CRD and writes it
// to the provided `dstPath`. The provided `crd` must only contain a single
// version, specified with `version`, since otherwise the KCL module will
// re-define the same schemas for each version in the same module. This function
// is concurrency-safe and will lock using `APIVersion` to prevent concurrent
// writes to the same KCL module.
func (g *genOpenAPI) FromCRDVersion(crd *unstructured.Unstructured, dstPath, version string) error {
	apiVersion := crd.GetAPIVersion()
	g.locker.Lock(apiVersion)
	defer g.locker.Unlock(apiVersion)

	tmpFile, err := os.CreateTemp(os.TempDir(), "kcl-swagger-")
	if err != nil {
		return fmt.Errorf("create temp file: %w: %w", kclerrors.ErrWriteFile, err)
	}

	crdData, err := yaml.Marshal(crd.UnstructuredContent())
	if err != nil {
		return fmt.Errorf("marshal CRD: %w: %w", kclerrors.ErrYAMLMarshal, err)
	}

	if _, err := tmpFile.Write(crdData); err != nil {
		return fmt.Errorf("write CRD to temp file: %w: %w", kclerrors.ErrWriteFile, err)
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

	opts := new(swaggergen.GenOpts)
	if err := opts.EnsureDefaults(); err != nil {
		return fmt.Errorf("failed to ensure default generator options: %w", err)
	}
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
