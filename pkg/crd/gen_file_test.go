package crd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/macropower/kclipper/pkg/crd"
)

func TestFileGenerator_FromPath(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		validate func(t *testing.T, crds []*unstructured.Unstructured, err error)
		filePath string
	}{
		"valid CRD file": {
			filePath: "testdata/valid-crd.yaml",
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 1)
				crd := crds[0]
				assert.Equal(t, "CustomResourceDefinition", crd.GetKind())
				assert.Equal(t, "apiextensions.k8s.io/v1", crd.GetAPIVersion())
				assert.Equal(t, "widgets.example.com", crd.GetName())
			},
		},
		"multiple CRDs in one file": {
			filePath: "testdata/multiple-crds.yaml",
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 2)

				// Check first CRD
				assert.Equal(t, "CustomResourceDefinition", crds[0].GetKind())
				assert.Equal(t, "apiextensions.k8s.io/v1", crds[0].GetAPIVersion())
				assert.Equal(t, "widgets.example.com", crds[0].GetName())

				// Check second CRD
				assert.Equal(t, "CustomResourceDefinition", crds[1].GetKind())
				assert.Equal(t, "apiextensions.k8s.io/v1", crds[1].GetAPIVersion())
				assert.Equal(t, "gadgets.example.com", crds[1].GetName())
			},
		},
		"mixed resources with CRDs": {
			filePath: "testdata/mixed-resources.yaml",
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 1)
				crd := crds[0]
				assert.Equal(t, "CustomResourceDefinition", crd.GetKind())
				assert.Equal(t, "apiextensions.k8s.io/v1", crd.GetAPIVersion())
				assert.Equal(t, "widgets.example.com", crd.GetName())
			},
		},
		"file not found": {
			filePath: "/path/to/nonexistent/file.yaml",
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to read file")
				assert.Nil(t, crds)
			},
		},
		"invalid YAML": {
			filePath: "testdata/invalid-yaml.yaml",
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "split yaml")
				assert.Nil(t, crds)
			},
		},
		"no CRDs in file": {
			filePath: "testdata/no-crds.yaml",
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				assert.Empty(t, crds)
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			generator := crd.NewFileGenerator()
			crds, err := generator.FromPath(tc.filePath)
			tc.validate(t, crds, err)
		})
	}
}

func TestFileGenerator_FromPaths(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		validate func(t *testing.T, crds []*unstructured.Unstructured, err error)
		paths    []string
	}{
		"no paths": {
			paths: []string{},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "no paths provided")
				assert.Nil(t, crds)
			},
		},
		"single path": {
			paths: []string{"testdata/valid-crd.yaml"},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 1)
				assert.Equal(t, "widgets.example.com", crds[0].GetName())
			},
		},
		"multiple paths": {
			paths: []string{"testdata/valid-crd.yaml", "testdata/multiple-crds.yaml"},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Len(t, crds, 3)

				// Extract all CRD names for easier validation
				names := make([]string, len(crds))
				for i, c := range crds {
					names[i] = c.GetName()
				}

				// Check that all expected CRDs are present (order may vary)
				assert.Contains(t, names, "widgets.example.com")
				assert.Contains(t, names, "gadgets.example.com")

				// Count occurrences of widgets.example.com (should appear twice - once from each file)
				widgetCount := 0
				for _, name := range names {
					if name == "widgets.example.com" {
						widgetCount++
					}
				}
				assert.Equal(t, 2, widgetCount)
			},
		},
		"one file with error": {
			paths: []string{"testdata/valid-crd.yaml", "/path/to/nonexistent/file.yaml"},
			validate: func(t *testing.T, crds []*unstructured.Unstructured, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to read CRDs from")
				assert.Contains(t, err.Error(), "nonexistent")
				assert.Nil(t, crds)
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			generator := crd.NewFileGenerator()
			crds, err := generator.FromPaths(tc.paths...)
			tc.validate(t, crds, err)
		})
	}
}

func TestDefaultFileGenerator(t *testing.T) {
	t.Parallel()

	// Ensure the default generator is properly initialized
	assert.NotNil(t, crd.DefaultFileGenerator)

	// Test with an existing testdata file
	crds, err := crd.DefaultFileGenerator.FromPath("testdata/valid-crd.yaml")
	require.NoError(t, err)
	require.Len(t, crds, 1)
	assert.Equal(t, "widgets.example.com", crds[0].GetName())
}
