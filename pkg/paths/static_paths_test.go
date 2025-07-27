// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0.

package paths_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/paths"
)

var tempDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testdataDir := filepath.Join(dir, "testdata")

	tempDir = filepath.Join(testdataDir, ".tmp")
	err := os.RemoveAll(tempDir)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(tempDir, 0o750)
	if err != nil {
		panic(err)
	}
}

func TestGetStaticPath_SameURLs(t *testing.T) {
	t.Parallel()

	stp := paths.NewStaticTempPaths(tempDir, paths.NewBase64PathEncoder())
	res1, err := stp.GetPath("https://localhost/test.txt")
	require.NoError(t, err)

	res2, err := stp.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetStaticPath_DifferentURLs(t *testing.T) {
	t.Parallel()

	stp := paths.NewStaticTempPaths(tempDir, paths.NewBase64PathEncoder())
	res1, err := stp.GetPath("https://localhost/test1.txt")
	require.NoError(t, err)

	res2, err := stp.GetPath("https://localhost/test2.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetStaticPath_SameURLsDifferentInstances(t *testing.T) {
	t.Parallel()

	stp1 := paths.NewStaticTempPaths(tempDir, paths.NewBase64PathEncoder())
	res1, err := stp1.GetPath("https://localhost/test.txt")
	require.NoError(t, err)

	stp2 := paths.NewStaticTempPaths(tempDir, paths.NewBase64PathEncoder())
	res2, err := stp2.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetStaticPathIfExists(t *testing.T) {
	t.Parallel()

	t.Run("does not exist", func(t *testing.T) {
		t.Parallel()

		stp := paths.NewStaticTempPaths(tempDir, paths.NewBase64PathEncoder())

		path := stp.GetPathIfExists("https://localhost/test.txt")
		assert.Empty(t, path)
	})
	t.Run("does exist", func(t *testing.T) {
		t.Parallel()

		stp := paths.NewStaticTempPaths(tempDir, paths.NewBase64PathEncoder())

		testFile, err := stp.GetPath("foo")
		require.NoError(t, err)

		err = os.WriteFile(testFile, []byte("test"), 0o600)
		require.NoError(t, err)

		key, err := stp.GetKey(testFile)
		require.NoError(t, err)
		assert.Equal(t, "foo", key)

		path := stp.GetPathIfExists(key)
		assert.NotEmpty(t, path)
	})
}

func TestGetStaticPaths_no_race(t *testing.T) {
	t.Parallel()

	stp := paths.NewStaticTempPaths(tempDir, paths.NewBase64PathEncoder())

	for range 100 {
		go func() {
			path := stp.GetPathIfExists("https://localhost/test.txt")
			assert.Empty(t, path)
		}()
	}
}
