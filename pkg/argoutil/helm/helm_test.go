package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/MacroPower/kclipper/pkg/argoutil/kube"
)

func template(h Helm, opts *TemplateOpts) ([]*unstructured.Unstructured, error) {
	out, _, err := h.Template(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to template Helm chart: %w", err)
	}
	ymls, err := kube.SplitYAML([]byte(out))
	if err != nil {
		return nil, fmt.Errorf("failed to split YAML: %w", err)
	}
	return ymls, nil
}

func TestHelmTemplateValues(t *testing.T) {
	t.Parallel()

	repoRoot := "./testdata/redis"
	repoRootAbs, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	h, err := NewHelmApp(repoRootAbs, []HelmRepository{}, false, "", "", "", false)
	require.NoError(t, err)
	valuesPath := filepath.Join(repoRootAbs, "values-production.yaml")

	valsFileBytes, err := os.ReadFile(valuesPath)
	require.NoError(t, err)

	vals := map[string]any{}
	err = yaml.Unmarshal(valsFileBytes, &vals)
	require.NoError(t, err)

	opts := TemplateOpts{
		Name:   "test",
		Values: vals,
	}
	objs, err := template(h, &opts)
	require.NoError(t, err)
	assert.Len(t, objs, 8)

	for _, obj := range objs {
		if obj.GetKind() == "Deployment" && obj.GetName() == "test-redis-slave" {
			var dep appsv1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dep)
			require.NoError(t, err)
			assert.Equal(t, int32(3), *dep.Spec.Replicas)
		}
	}
}

func TestHelmTemplateReleaseNameOverwrite(t *testing.T) {
	t.Parallel()

	h, err := NewHelmApp("./testdata/redis", nil, false, "", "", "", false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{Name: "my-release"})
	require.NoError(t, err)
	assert.Len(t, objs, 5)

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			require.NoError(t, err)
			assert.Equal(t, "my-release-redis-master", stateful.ObjectMeta.Name)
		}
	}
}

func TestHelmTemplateReleaseName(t *testing.T) {
	t.Parallel()

	h, err := NewHelmApp("./testdata/redis", nil, false, "", "", "", false)
	require.NoError(t, err)
	objs, err := template(h, &TemplateOpts{Name: "test"})
	require.NoError(t, err)
	assert.Len(t, objs, 5)

	for _, obj := range objs {
		if obj.GetKind() == "StatefulSet" {
			var stateful appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &stateful)
			require.NoError(t, err)
			assert.Equal(t, "test-redis-master", stateful.ObjectMeta.Name)
		}
	}
}

func TestAPIVersions(t *testing.T) {
	t.Parallel()

	h, err := NewHelmApp("./testdata/api-versions", nil, false, "", "", "", false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{Name: "test-api-versions"})
	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.Equal(t, "sample/v1", objs[0].GetAPIVersion())

	objs, err = template(h, &TemplateOpts{Name: "test-api-versions", APIVersions: []string{"sample/v2"}})
	require.NoError(t, err)
	require.Len(t, objs, 1)
	assert.Equal(t, "sample/v2", objs[0].GetAPIVersion())
}

func TestSkipCrds(t *testing.T) {
	t.Parallel()

	h, err := NewHelmApp("./testdata/crds", nil, false, "", "", "", false)
	require.NoError(t, err)

	objs, err := template(h, &TemplateOpts{Name: "test-skip-crds", SkipCrds: false})
	require.NoError(t, err)
	require.Len(t, objs, 1)

	objs, err = template(h, &TemplateOpts{Name: "test-skip-crds"})
	require.NoError(t, err)
	require.Len(t, objs, 1)

	objs, err = template(h, &TemplateOpts{Name: "test-skip-crds", SkipCrds: true})
	require.NoError(t, err)
	require.Empty(t, objs)
}
