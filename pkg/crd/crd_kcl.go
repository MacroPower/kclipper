package crd

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/macropower/kclipper/pkg/kclgen"
)

// KCLPackage represents a KCL package that contains CRD schemas.
type KCLPackage struct {
	path string
	crds []*unstructured.Unstructured
}

// NewKCLPackage creates a new [KCLPackage] at the specified path, containing
// the provided CRD resources.
func NewKCLPackage(crds []*unstructured.Unstructured, path string) *KCLPackage {
	return &KCLPackage{
		crds: crds,
		path: path,
	}
}

// GenerateC generates KCL schemas from Kubernetes CRDs and writes them to the
// package path concurrently. Any errors will be collected and will only be
// returned after all processing is complete.
func (s *KCLPackage) GenerateC(ctx context.Context) error {
	crdCount := len(s.crds)
	if crdCount == 0 {
		return nil
	}

	var wg sync.WaitGroup

	errChan := make(chan error, crdCount)

	for _, uCRD := range s.crds {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		default:
			// Continue processing.
		}

		wg.Go(func() {
			err := s.writeToKCLSchema(uCRD)
			if err != nil {
				errChan <- err
			}
		})
	}

	// Close errChan when all goroutines complete.
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect errors.
	var merr error
	for err := range errChan {
		if err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	if merr != nil {
		return fmt.Errorf("failed to generate KCL from CRDs: %w", merr)
	}

	return nil
}

// Generate generates KCL schemas from Kubernetes CRDs and writes them to
// the package path.
func (s *KCLPackage) Generate() error {
	for _, u := range s.crds {
		err := s.writeToKCLSchema(u)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *KCLPackage) writeToKCLSchema(uCRD *unstructured.Unstructured) error {
	crdVersions, err := SplitCRDVersions(uCRD)
	if err != nil {
		return fmt.Errorf("failed to split CRD versions: %w", err)
	}

	var merr error
	for version, v := range crdVersions {
		err := kclgen.GenOpenAPI.FromCRDVersion(&v, s.path, version)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("%s: %w", v.GetAPIVersion(), err))
		}
	}

	if merr != nil {
		return multierror.Prefix(merr, ErrGenerateKCL.Error()+":") //nolint:wrapcheck // Multierror
	}

	return nil
}
