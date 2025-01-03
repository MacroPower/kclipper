// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0

//nolint:testpackage
package pathutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPath_SameURLs(t *testing.T) {
	t.Parallel()

	paths := NewRandomizedTempPaths(os.TempDir())
	res1, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetPath_DifferentURLs(t *testing.T) {
	t.Parallel()

	paths := NewRandomizedTempPaths(os.TempDir())
	res1, err := paths.GetPath("https://localhost/test1.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test2.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetPath_SameURLsDifferentInstances(t *testing.T) {
	t.Parallel()

	paths1 := NewRandomizedTempPaths(os.TempDir())
	res1, err := paths1.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	paths2 := NewRandomizedTempPaths(os.TempDir())
	res2, err := paths2.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetPathIfExists(t *testing.T) {
	t.Parallel()

	t.Run("does not exist", func(t *testing.T) {
		t.Parallel()
		paths := NewRandomizedTempPaths(os.TempDir())
		path := paths.GetPathIfExists("https://localhost/test.txt")
		assert.Empty(t, path)
	})
	t.Run("does exist", func(t *testing.T) {
		t.Parallel()
		paths := NewRandomizedTempPaths(os.TempDir())
		_, err := paths.GetPath("https://localhost/test.txt")
		require.NoError(t, err)
		path := paths.GetPathIfExists("https://localhost/test.txt")
		assert.NotEmpty(t, path)
	})
}

func TestGetPaths_no_race(t *testing.T) {
	t.Parallel()

	paths := NewRandomizedTempPaths(os.TempDir())
	go func() {
		path, err := paths.GetPath("https://localhost/test.txt")
		assert.NoError(t, err)
		assert.NotEmpty(t, path)
	}()
	go func() {
		paths.GetPaths()
	}()
}
