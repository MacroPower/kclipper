package crd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/MacroPower/kclipper/pkg/crd"
)

func TestSplitCRDVersions(t *testing.T) {
	t.Parallel()

	// Error test cases
	errorTcs := map[string]struct {
		crdObject map[string]any
		errMsg    string
	}{
		"missing versions": {
			crdObject: map[string]any{
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
			},
			errMsg: "invalid spec.versions field",
		},
		"invalid version structure": {
			crdObject: map[string]any{
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
			},
			errMsg: "invalid spec.versions[] field",
		},
		"missing version name": {
			crdObject: map[string]any{
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
			},
			errMsg: "invalid spec.versions[].name field",
		},
		"missing spec field": {
			crdObject: map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": "missing-spec.example.com",
				},
				// Missing spec field
			},
			errMsg: "invalid spec field",
		},
	}

	for name, tc := range errorTcs {
		t.Run("error_"+name, func(t *testing.T) {
			t.Parallel()

			c := &unstructured.Unstructured{Object: tc.crdObject}
			_, err := crd.SplitCRDVersions(c)
			require.Error(t, err)
			require.ErrorIs(t, err, crd.ErrInvalidFormat)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}

	// Success test cases
	successTcs := map[string]struct {
		crdObject       map[string]any
		expectedSchemas map[string]bool
		expectedCount   int
	}{
		"single version": {
			crdObject: map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": "widgets.example.com",
				},
				"spec": map[string]any{
					"group": "example.com",
					"names": map[string]any{
						"kind":     "Widget",
						"plural":   "widgets",
						"singular": "widget",
					},
					"scope": "Namespaced",
					"versions": []any{
						map[string]any{
							"name":    "v1",
							"served":  true,
							"storage": true,
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
			expectedCount: 1,
			expectedSchemas: map[string]bool{
				"v1": true,
			},
		},
		"multiple versions": {
			crdObject: map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": "widgets.example.com",
				},
				"spec": map[string]any{
					"group": "example.com",
					"names": map[string]any{
						"kind":     "Widget",
						"plural":   "widgets",
						"singular": "widget",
					},
					"scope": "Namespaced",
					"versions": []any{
						map[string]any{
							"name":    "v1",
							"served":  true,
							"storage": true,
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"type": "object",
								},
							},
						},
						map[string]any{
							"name":    "v2",
							"served":  true,
							"storage": false,
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"type": "object",
								},
							},
						},
						map[string]any{
							"name":    "v3alpha1",
							"served":  false,
							"storage": false,
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
			expectedCount: 3,
			expectedSchemas: map[string]bool{
				"v1":       true,
				"v2":       true,
				"v3alpha1": true,
			},
		},
	}

	for name, tc := range successTcs {
		t.Run("success_"+name, func(t *testing.T) {
			t.Parallel()

			c := &unstructured.Unstructured{Object: tc.crdObject}
			versions, err := crd.SplitCRDVersions(c)
			require.NoError(t, err)
			assert.Len(t, versions, tc.expectedCount)

			for expectedVersion := range tc.expectedSchemas {
				assert.Contains(t, versions, expectedVersion, "Should contain version %s", expectedVersion)

				// Verify the version field contains only the expected version
				crdVersion := versions[expectedVersion]
				spec, ok := crdVersion.Object["spec"].(map[string]any)
				require.True(t, ok, "Should have a spec field")

				versionsArray, ok := spec["versions"].([]any)
				require.True(t, ok, "Should have versions array")
				require.Len(t, versionsArray, 1, "Should have exactly one version")

				versionObj, ok := versionsArray[0].(map[string]any)
				require.True(t, ok, "Version should be a map")
				assert.Equal(t, expectedVersion, versionObj["name"], "Version name should match")
			}
		})
	}
}
