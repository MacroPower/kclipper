// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0

package pathutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/pathutil"
)

func Test_resolveSymlinkRecursive(t *testing.T) {
	t.Parallel()

	testsDir, err := filepath.Abs("./testdata")
	if err != nil {
		panic(err)
	}
	t.Run("Resolve non-symlink", func(t *testing.T) {
		t.Parallel()
		r, err := pathutil.ResolveSymbolicLinkRecursive(testsDir+"/foo", 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foo", r)
	})
	t.Run("Successfully resolve symlink", func(t *testing.T) {
		t.Parallel()
		r, err := pathutil.ResolveSymbolicLinkRecursive(testsDir+"/bar", 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foo", r)
	})
	t.Run("Do not allow symlink at all", func(t *testing.T) {
		t.Parallel()
		r, err := pathutil.ResolveSymbolicLinkRecursive(testsDir+"/bar", 0)
		require.Error(t, err)
		assert.Equal(t, "", r)
	})
	t.Run("Error because too nested symlink", func(t *testing.T) {
		t.Parallel()
		r, err := pathutil.ResolveSymbolicLinkRecursive(testsDir+"/bam", 2)
		require.Error(t, err)
		assert.Equal(t, "", r)
	})
	t.Run("No such file or directory", func(t *testing.T) {
		t.Parallel()
		r, err := pathutil.ResolveSymbolicLinkRecursive(testsDir+"/foobar", 2)
		require.NoError(t, err)
		assert.Equal(t, testsDir+"/foobar", r)
	})
}

var allowedRemoteProtocols = []string{"http", "https"}

func Test_resolveFilePath(t *testing.T) {
	t.Parallel()

	t.Run("Resolve normal relative path into absolute path", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "baz/bim.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/bar/baz/bim.yaml", p.String())
	})
	t.Run("Resolve normal relative path into absolute path", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "baz/../../bim.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/bim.yaml", p.String())
	})
	t.Run("Error on path resolving outside repository root", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "baz/../../../bim.yaml", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Return verbatim URL", func(t *testing.T) {
		t.Parallel()
		url := "https://some.where/foo,yaml"
		p, err := pathutil.ResolveFilePathOrURL(
			"/foo/bar", "/foo", url, allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, url, p.String())
	})
	t.Run("URL scheme not allowed", func(t *testing.T) {
		t.Parallel()
		url := "file:///some.where/foo,yaml"
		p, err := pathutil.ResolveFilePathOrURL(
			"/foo/bar", "/foo", url, allowedRemoteProtocols)
		require.Error(t, err)
		assert.Equal(t, "", p.String())
	})
	t.Run("Implicit URL by absolute path", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			"/foo/bar", "/foo", "/baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/baz.yaml", p.String())
	})
	t.Run("Relative app path", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			".", "/foo", "/baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, "/foo/baz.yaml", p.String())
	})
	t.Run("Relative repo path", func(t *testing.T) {
		t.Parallel()
		c, err := os.Getwd()
		require.NoError(t, err)
		p, err := pathutil.ResolveFilePathOrURL(
			".", ".", "baz.yaml", allowedRemoteProtocols)
		require.NoError(t, err)
		assert.Equal(t, c+"/baz.yaml", p.String())
	})
	t.Run("Overlapping root prefix without trailing slash", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			".", "/foo", "../foo2/baz.yaml", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Overlapping root prefix with trailing slash", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			".", "/foo/", "../foo2/baz.yaml", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Garbage input as values file", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			".", "/foo/", "kfdj\\ks&&&321209.,---e32908923%$§!\"", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("NUL-byte path input as values file", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFilePathOrURL(
			".", "/foo/", "\000", allowedRemoteProtocols)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outside repository root")
		assert.Equal(t, "", p.String())
	})
	t.Run("Resolve root path into absolute path - jsonnet library path", func(t *testing.T) {
		t.Parallel()
		p, err := pathutil.ResolveFileOrDirectoryPath("/foo", "/foo", "./")
		require.NoError(t, err)
		assert.Equal(t, "/foo", string(p))
	})
}
