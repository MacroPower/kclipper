// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/paths"
)

func Test_resolveSymlinkRecursive(t *testing.T) {
	t.Parallel()

	testsDir, err := filepath.Abs("./testdata")
	if err != nil {
		panic(err)
	}

	t.Run("Resolve non-symlink", func(t *testing.T) {
		t.Parallel()

		r, err := paths.ResolveSymbolicLinkRecursive(testsDir+"/foo", 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foo", r)
	})
	t.Run("Successfully resolve symlink", func(t *testing.T) {
		t.Parallel()

		r, err := paths.ResolveSymbolicLinkRecursive(testsDir+"/bar", 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foo", r)
	})
	t.Run("Do not allow symlink at all", func(t *testing.T) {
		t.Parallel()

		r, err := paths.ResolveSymbolicLinkRecursive(testsDir+"/bar", 0)
		require.Error(t, err)
		assert.Equal(t, "", r)
	})
	t.Run("Error because too nested symlink", func(t *testing.T) {
		t.Parallel()

		r, err := paths.ResolveSymbolicLinkRecursive(testsDir+"/bam", 2)
		require.Error(t, err)
		assert.Equal(t, "", r)
	})
	t.Run("No such file or directory", func(t *testing.T) {
		t.Parallel()

		r, err := paths.ResolveSymbolicLinkRecursive(testsDir+"/foobar", 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foobar", r)
	})
}

func Test_resolveFilePath(t *testing.T) {
	t.Parallel()

	allowedRemoteProtocols := []string{"http", "https"}

	t.Run("Resolve normal relative path into absolute path", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "baz/bim.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/bar/baz/bim.yaml", p.String())
	})
	t.Run("Resolve normal relative path into absolute path", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "baz/../../bim.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/bim.yaml", p.String())
	})
	t.Run("Error on path resolving outside repository root", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "baz/../../../bim.yaml", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Return verbatim URL", func(t *testing.T) {
		t.Parallel()

		url := "https://some.where/foo,yaml"
		p, err := paths.ResolveFilePathOrURL(
			"/foo/bar", "/foo", url, allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, url, p.String())
	})
	t.Run("URL scheme not allowed", func(t *testing.T) {
		t.Parallel()

		url := "file:///some.where/foo,yaml"
		p, err := paths.ResolveFilePathOrURL(
			"/foo/bar", "/foo", url, allowedRemoteProtocols)
		require.Error(t, err)
		assert.Equal(t, "", p.String())
	})
	t.Run("Implicit URL by absolute path", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "/baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/baz.yaml", p.String())
	})
	t.Run("Relative app path", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			".", "/foo", "/baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/baz.yaml", p.String())
	})
	t.Run("Relative repo path", func(t *testing.T) {
		t.Parallel()

		c, err := os.Getwd()
		require.NoError(t, err)
		p, err := paths.ResolveFilePathOrURL(
			".", ".", "baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, c+"/baz.yaml", p.String())
	})
	t.Run("Overlapping root prefix without trailing slash", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			".", "/foo", "../foo2/baz.yaml", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Overlapping root prefix with trailing slash", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			".", "/foo/", "../foo2/baz.yaml", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Garbage input as values file", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			".", "/foo/", "kfdj\\ks&&&321209.,---e32908923%$§!\"", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("NUL-byte path input as values file", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFilePathOrURL(
			".", "/foo/", "\000", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Resolve root path into absolute path - jsonnet library path", func(t *testing.T) {
		t.Parallel()

		p, err := paths.ResolveFileOrDirectoryPath("/foo", "/foo", "./")
		require.NoError(t, err)
		assert.Equal(t, "/foo", string(p))
	})
}
