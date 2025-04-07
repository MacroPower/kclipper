package paths_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/kclerrors"
	"github.com/MacroPower/kclipper/pkg/paths"
)

func TestFindTopPkgRoot(t *testing.T) {
	t.Parallel()

	testDataDir, err := filepath.Abs("testdata/kcl")
	require.NoError(t, err)

	tcs := map[string]struct {
		err  error
		path string
		want string
	}{
		"subpkg": {
			path: filepath.Join(testDataDir, "./pkg/subpkg"),
			want: filepath.Join(testDataDir, "."),
		},
		"pkg": {
			path: filepath.Join(testDataDir, "./pkg"),
			want: filepath.Join(testDataDir, "."),
		},
		"root": {
			path: filepath.Join(testDataDir, "."),
			want: filepath.Join(testDataDir, "."),
		},
		"not a module": {
			path: ".",
			err:  kclerrors.ErrFileNotFound,
		},
		"out of bounds": {
			path: "../../../",
			err:  kclerrors.ErrResolvedOutsideRepo,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repoRoot, err := paths.FindRepoRoot(".")
			require.NoError(t, err)

			got, err := paths.FindTopPkgRoot(repoRoot, tc.path)
			if tc.err == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.err)
			}

			require.Equal(t, tc.want, got)
		})
	}
}

func TestFindRepoRoot(t *testing.T) {
	t.Parallel()

	want, err := filepath.Abs("../../")
	require.NoError(t, err)

	got, err := paths.FindRepoRoot(".")
	require.NoError(t, err)

	require.Equal(t, want, got)
}
