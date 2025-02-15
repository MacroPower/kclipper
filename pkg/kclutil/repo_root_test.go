package kclutil_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/kclutil"
)

func TestFindTopPkgRoot(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		err  error
		path string
		want string
	}{
		"subpkg": {
			path: filepath.Join(testDataDir, "./kcl/pkg/subpkg"),
			want: filepath.Join(testDataDir, "./kcl"),
		},
		"pkg": {
			path: filepath.Join(testDataDir, "./kcl/pkg"),
			want: filepath.Join(testDataDir, "./kcl"),
		},
		"root": {
			path: filepath.Join(testDataDir, "./kcl"),
			want: filepath.Join(testDataDir, "./kcl"),
		},
		"not a module": {
			path: ".",
			err:  kclutil.ErrFileNotFound,
		},
		"out of bounds": {
			path: "../../../",
			err:  kclutil.ErrResolvedOutsideRepo,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repoRoot, err := kclutil.FindRepoRoot(".")
			require.NoError(t, err)

			got, err := kclutil.FindTopPkgRoot(repoRoot, tc.path)
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

	got, err := kclutil.FindRepoRoot(".")
	require.NoError(t, err)

	require.Equal(t, want, got)
}
