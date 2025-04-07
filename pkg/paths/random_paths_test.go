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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/paths"
)

func TestGetRandomizedPath_SameURLs(t *testing.T) {
	t.Parallel()

	rtp := paths.NewRandomizedTempPaths(t.TempDir())
	res1, err := rtp.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	res2, err := rtp.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetRandomizedPath_DifferentURLs(t *testing.T) {
	t.Parallel()

	rtp := paths.NewRandomizedTempPaths(t.TempDir())
	res1, err := rtp.GetPath("https://localhost/test1.txt")
	require.NoError(t, err)
	res2, err := rtp.GetPath("https://localhost/test2.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetRandomizedPath_SameURLsDifferentInstances(t *testing.T) {
	t.Parallel()

	rtp1 := paths.NewRandomizedTempPaths(t.TempDir())
	res1, err := rtp1.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	rtp2 := paths.NewRandomizedTempPaths(t.TempDir())
	res2, err := rtp2.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetRandomizedPathIfExists(t *testing.T) {
	t.Parallel()

	t.Run("does not exist", func(t *testing.T) {
		t.Parallel()
		rtp := paths.NewRandomizedTempPaths(t.TempDir())
		path := rtp.GetPathIfExists("https://localhost/test.txt")
		assert.Empty(t, path)
	})
	t.Run("does exist", func(t *testing.T) {
		t.Parallel()
		rtp := paths.NewRandomizedTempPaths(t.TempDir())
		_, err := rtp.GetPath("https://localhost/test.txt")
		require.NoError(t, err)
		path := rtp.GetPathIfExists("https://localhost/test.txt")
		assert.NotEmpty(t, path)
	})
}

func TestGetRandomizedPaths_no_race(t *testing.T) {
	t.Parallel()

	rtp := paths.NewRandomizedTempPaths(t.TempDir())
	go func() {
		path, err := rtp.GetPath("https://localhost/test.txt")
		assert.NoError(t, err)
		assert.NotEmpty(t, path)
	}()
	go func() {
		rtp.GetPaths()
	}()
}
