package crd

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/macropower/kclipper/pkg/kclerrors"
	"github.com/macropower/kclipper/pkg/kclgen"
	"github.com/macropower/kclipper/pkg/kube"
)

// GenerateKCL generates KCL schemas from Kubernetes CRDs and writes them to
// the given path concurrently. Any errors are collected and returned after all
// processing is complete.
func GenerateKCL(ctx context.Context, crds []kube.Object, path string) error {
	if len(crds) == 0 {
		return nil
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, uCRD := range crds {
		select {
		case <-ctx.Done():
			wg.Wait()

			return fmt.Errorf("context canceled: %w", ctx.Err())

		default:
			// Continue processing.
		}

		wg.Go(func() {
			err := writeToKCLSchema(uCRD, path)
			if err != nil {
				mu.Lock()
				defer mu.Unlock()

				errs = append(errs, err)
			}
		})
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("generate KCL from CRDs: %w", errors.Join(errs...))
	}

	return nil
}

func writeToKCLSchema(obj kube.Object, path string) error {
	crdVersions, err := SplitCRDVersions(obj)
	if err != nil {
		return fmt.Errorf("split CRD versions: %w", err)
	}

	var errs []error
	for version, v := range crdVersions {
		err := kclgen.GenOpenAPI.FromCRDVersion(v, path, version)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", v.GetAPIVersion(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %w", kclerrors.ErrGenerateKCL, errors.Join(errs...))
	}

	return nil
}
