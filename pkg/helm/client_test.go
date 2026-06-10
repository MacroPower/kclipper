package helm_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/helm"
	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/paths"
)

// chartArchive builds a minimal Helm chart .tgz in memory.
func chartArchive(t *testing.T, name, version string) []byte {
	t.Helper()

	files := []struct {
		path    string
		content string
	}{
		{
			path:    name + "/Chart.yaml",
			content: fmt.Sprintf("apiVersion: v2\nname: %s\nversion: %s\n", name, version),
		},
		{
			path:    name + "/values.yaml",
			content: "replicas: 1\n",
		},
		{
			path: name + "/templates/configmap.yaml",
			content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n" +
				"  name: {{ .Release.Name }}\ndata:\n" +
				"  version: {{ .Chart.Version | quote }}\n",
		},
	}

	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, f := range files {
		err := tw.WriteHeader(&tar.Header{
			Name:     f.path,
			Mode:     0o644,
			Size:     int64(len(f.content)),
			Typeflag: tar.TypeReg,
		})
		require.NoError(t, err)

		_, err = tw.Write([]byte(f.content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	return buf.Bytes()
}

// newChartServer serves a Helm repository index and chart archives for the
// given chart name and versions over HTTP.
func newChartServer(t *testing.T, name string, versions []string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	var index strings.Builder

	index.WriteString("apiVersion: v1\nentries:\n")
	fmt.Fprintf(&index, "  %s:\n", name)

	for _, version := range versions {
		index.WriteString("    - apiVersion: v2\n")
		fmt.Fprintf(&index, "      name: %s\n", name)
		fmt.Fprintf(&index, "      version: %s\n", version)
		index.WriteString("      urls:\n")
		fmt.Fprintf(&index, "        - %s/%s-%s.tgz\n", srv.URL, name, version)

		archive := chartArchive(t, name, version)
		mux.HandleFunc(fmt.Sprintf("/%s-%s.tgz", name, version),
			func(w http.ResponseWriter, _ *http.Request) {
				_, err := w.Write(archive)
				assert.NoError(t, err)
			})
	}

	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(index.String()))
		assert.NoError(t, err)
	})

	return srv
}

func newTestClient(t *testing.T) *helm.Client {
	t.Helper()

	return helm.MustNewClient(
		paths.NewStaticTempPaths(t.TempDir(), paths.NewBase64PathEncoder()),
		"test",
	)
}

func TestClientPullVersions(t *testing.T) {
	t.Parallel()

	srv := newChartServer(t, "test-chart", []string{"1.2.3", "1.2.5", "1.3.0"})

	tcs := map[string]struct {
		version string
		want    string
	}{
		"exact version": {
			version: "1.2.3",
			want:    "1.2.3",
		},
		"tilde range": {
			version: "~1.2.0",
			want:    "1.2.5",
		},
		"caret range": {
			version: "^1.2.3",
			want:    "1.3.0",
		},
		"x range": {
			version: "1.2.x",
			want:    "1.2.5",
		},
		"latest when empty": {
			version: "",
			want:    "1.3.0",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := newTestClient(t)

			pulledChart, err := client.Pull(
				t.Context(), "test-chart", srv.URL, tc.version, helmrepo.DefaultManager,
			)
			require.NoError(t, err)

			loadedChart, err := pulledChart.Load(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tc.want, loadedChart.Metadata.Version)
		})
	}
}

func TestClientPullDependencyVersions(t *testing.T) {
	t.Parallel()

	srv := newChartServer(t, "dep-chart", []string{"1.2.3", "1.2.5", "1.3.0"})

	tcs := map[string]struct {
		version string
		want    string
	}{
		"exact version": {
			version: "1.2.3",
			want:    "1.2.3",
		},
		"tilde range": {
			version: "~1.2.0",
			want:    "1.2.5",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repoRoot := t.TempDir()
			chartDir := filepath.Join(repoRoot, "charts", "parent-chart")
			require.NoError(t, os.MkdirAll(chartDir, 0o700))

			chartYAML := fmt.Sprintf(
				"apiVersion: v2\nname: parent-chart\nversion: 0.1.0\ndependencies:\n"+
					"  - name: dep-chart\n    version: %q\n    repository: %s\n",
				tc.version, srv.URL,
			)
			require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("{}\n"), 0o600))

			repoMgr := helmrepo.NewManager(helmrepo.WithAllowedPaths(repoRoot, repoRoot))
			client := newTestClient(t)

			pulledChart, err := client.Pull(t.Context(), "parent-chart", "./charts", "", repoMgr)
			require.NoError(t, err)

			loadedChart, err := pulledChart.Load(t.Context())
			require.NoError(t, err)

			deps := loadedChart.Dependencies()
			require.Len(t, deps, 1)
			assert.Equal(t, "dep-chart", deps[0].Name())
			assert.Equal(t, tc.want, deps[0].Metadata.Version)
		})
	}
}
