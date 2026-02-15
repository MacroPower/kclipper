package kube_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/macropower/kclipper/pkg/kube"
)

func TestObject_GetKind(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		obj  kube.Object
		want string
	}{
		"valid kind": {
			obj: kube.Object{
				"kind": "Pod",
			},
			want: "Pod",
		},
		"missing kind": {
			obj:  kube.Object{},
			want: "",
		},
		"nil object": {
			obj:  nil,
			want: "",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.obj.GetKind()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetAPIVersion(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		obj  kube.Object
		want string
	}{
		"valid apiVersion": {
			obj: kube.Object{
				"apiVersion": "v1",
			},
			want: "v1",
		},
		"valid apiVersion with group": {
			obj: kube.Object{
				"apiVersion": "apps/v1",
			},
			want: "apps/v1",
		},
		"missing apiVersion": {
			obj:  kube.Object{},
			want: "",
		},
		"nil object": {
			obj:  nil,
			want: "",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.obj.GetAPIVersion()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_GetName(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		obj  kube.Object
		want string
	}{
		"valid name": {
			obj: kube.Object{
				"metadata": map[string]any{
					"name": "my-pod",
				},
			},
			want: "my-pod",
		},
		"missing metadata": {
			obj:  kube.Object{},
			want: "",
		},
		"missing name in metadata": {
			obj: kube.Object{
				"metadata": map[string]any{},
			},
			want: "",
		},
		"nil object": {
			obj:  nil,
			want: "",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.obj.GetName()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObject_DeepCopy(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		obj kube.Object
	}{
		"simple object": {
			obj: kube.Object{
				"kind":       "Pod",
				"apiVersion": "v1",
			},
		},
		"nested object": {
			obj: kube.Object{
				"metadata": map[string]any{
					"name": "test",
					"labels": map[string]any{
						"app": "test",
					},
				},
			},
		},
		"with slice": {
			obj: kube.Object{
				"items": []any{"a", "b", "c"},
			},
		},
		"with nested slice": {
			obj: kube.Object{
				"items": []any{
					map[string]any{"key": "val1"},
					map[string]any{"key": "val2"},
				},
			},
		},
		"nil object": {
			obj: nil,
		},
		"empty object": {
			obj: kube.Object{},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.obj.DeepCopy()

			// Check that deep copy matches original
			assert.Equal(t, tc.obj, got)

			if tc.obj == nil {
				assert.Nil(t, got)
				return
			}

			// Verify deep independence: modifying copy doesn't affect original
			if metadata, ok := got["metadata"].(map[string]any); ok {
				metadata["modified"] = "true"
				if origMetadata, exists := tc.obj["metadata"].(map[string]any); exists {
					_, hasModified := origMetadata["modified"]
					assert.False(t, hasModified, "Original should not be modified")
				}
			}

			// Test another mutation
			got["new_field"] = "new_value"
			_, exists := tc.obj["new_field"]
			assert.False(t, exists, "Original should not have new field")
		})
	}
}

func TestObject_IsCRD(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		obj  kube.Object
		want bool
	}{
		"valid CRD v1": {
			obj: kube.Object{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
			},
			want: true,
		},
		"valid CRD v1beta1": {
			obj: kube.Object{
				"apiVersion": "apiextensions.k8s.io/v1beta1",
				"kind":       "CustomResourceDefinition",
			},
			want: true,
		},
		"wrong apiVersion": {
			obj: kube.Object{
				"apiVersion": "v1",
				"kind":       "CustomResourceDefinition",
			},
			want: false,
		},
		"wrong kind": {
			obj: kube.Object{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "Pod",
			},
			want: false,
		},
		"regular pod": {
			obj: kube.Object{
				"apiVersion": "v1",
				"kind":       "Pod",
			},
			want: false,
		},
		"empty object": {
			obj:  kube.Object{},
			want: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.obj.IsCRD()
			assert.Equal(t, tc.want, got)
		})
	}
}
