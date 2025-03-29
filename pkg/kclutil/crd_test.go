package kclutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

func TestSplitCRDVersions(t *testing.T) {
	t.Parallel()

	t.Run("FromCRD_InvalidCRD", func(t *testing.T) {
		t.Parallel()

		// Create an invalid CRD missing versions
		invalidCRD := map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]any{
				"name": "invalid.example.com",
			},
			"spec": map[string]any{
				"group": "example.com",
				"names": map[string]any{
					"kind": "Invalid",
				},
				// Missing versions field
			},
		}

		crd := &unstructured.Unstructured{Object: invalidCRD}
		testDir := t.TempDir()

		err := kclutil.GenOpenAPI.FromCRD(crd, testDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to split CRD versions")
	})

	t.Run("FromCRD_InvalidVersion", func(t *testing.T) {
		t.Parallel()

		// Create a CRD with invalid version structure
		invalidVersionCRD := map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]any{
				"name": "invalid.example.com",
			},
			"spec": map[string]any{
				"group": "example.com",
				"names": map[string]any{
					"kind": "Invalid",
				},
				"versions": []any{
					"not-a-map", // This should be a map
				},
			},
		}

		crd := &unstructured.Unstructured{Object: invalidVersionCRD}
		testDir := t.TempDir()

		err := kclutil.GenOpenAPI.FromCRD(crd, testDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid spec.versions[] field")
	})

	t.Run("FromCRD_MissingName", func(t *testing.T) {
		t.Parallel()

		// Create a CRD with a version missing a name
		missingNameCRD := map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]any{
				"name": "missing-name.example.com",
			},
			"spec": map[string]any{
				"group": "example.com",
				"names": map[string]any{
					"kind": "MissingName",
				},
				"versions": []any{
					map[string]any{
						// Missing "name" field
						"served":  true,
						"storage": true,
					},
				},
			},
		}

		crd := &unstructured.Unstructured{Object: missingNameCRD}
		testDir := t.TempDir()

		err := kclutil.GenOpenAPI.FromCRD(crd, testDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid spec.versions[].name field")
	})
}
