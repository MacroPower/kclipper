// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0.

package pathutil_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/pathutil"
)

var tempDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testdataDir := filepath.Join(dir, "testdata")

	tempDir = filepath.Join(testdataDir, ".tmp")
	if err := os.RemoveAll(tempDir); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		panic(err)
	}
}

func TestGetStaticPath_SameURLs(t *testing.T) {
	t.Parallel()

	paths := pathutil.NewStaticTempPaths(tempDir, pathutil.NewBase64PathEncoder())
	res1, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetStaticPath_DifferentURLs(t *testing.T) {
	t.Parallel()

	paths := pathutil.NewStaticTempPaths(tempDir, pathutil.NewBase64PathEncoder())
	res1, err := paths.GetPath("https://localhost/test1.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test2.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetStaticPath_SameURLsDifferentInstances(t *testing.T) {
	t.Parallel()

	paths1 := pathutil.NewStaticTempPaths(tempDir, pathutil.NewBase64PathEncoder())
	res1, err := paths1.GetPath("https://localhost/test.txt")
	require.NoError(t, err)

	paths2 := pathutil.NewStaticTempPaths(tempDir, pathutil.NewBase64PathEncoder())
	res2, err := paths2.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetStaticPathIfExists(t *testing.T) {
	t.Parallel()

	t.Run("does not exist", func(t *testing.T) {
		t.Parallel()

		paths := pathutil.NewStaticTempPaths(tempDir, pathutil.NewBase64PathEncoder())

		path := paths.GetPathIfExists("https://localhost/test.txt")
		assert.Empty(t, path)
	})
	t.Run("does exist", func(t *testing.T) {
		t.Parallel()

		paths := pathutil.NewStaticTempPaths(tempDir, pathutil.NewBase64PathEncoder())

		testFile, err := paths.GetPath("foo")
		require.NoError(t, err)
		err = os.WriteFile(testFile, []byte("test"), 0o600)
		require.NoError(t, err)

		key, err := paths.GetKey(testFile)
		require.NoError(t, err)
		assert.Equal(t, "foo", key)

		path := paths.GetPathIfExists(key)
		assert.NotEmpty(t, path)
	})
}

func TestGetStaticPaths_no_race(t *testing.T) {
	t.Parallel()

	paths := pathutil.NewStaticTempPaths(tempDir, pathutil.NewBase64PathEncoder())

	for range 100 {
		go func() {
			path := paths.GetPathIfExists("https://localhost/test.txt")
			assert.Empty(t, path)
		}()
	}
}
