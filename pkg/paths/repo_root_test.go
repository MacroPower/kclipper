package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/kclerrors"
	"github.com/macropower/kclipper/pkg/paths"
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

	t.Run("current repo", func(t *testing.T) {
		t.Parallel()

		want, err := filepath.Abs("../../")
		require.NoError(t, err)

		got, err := paths.FindRepoRoot(".")
		require.NoError(t, err)

		require.Equal(t, want, got)
	})

	t.Run("normal git directory", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		gitDir := filepath.Join(tmp, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))

		got, err := paths.FindRepoRoot(tmp)
		require.NoError(t, err)
		require.Equal(t, tmp, got)
	})

	t.Run("worktree with absolute gitdir", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()

		// Create the real git dir that the worktree points to.
		realGitDir := filepath.Join(tmp, "main-repo", ".git", "worktrees", "wt")
		require.NoError(t, os.MkdirAll(realGitDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(realGitDir, "HEAD"), []byte("ref: refs/heads/feature\n"), 0o644))

		// Create the worktree directory with a .git file.
		wtDir := filepath.Join(tmp, "worktree")
		require.NoError(t, os.MkdirAll(wtDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: "+realGitDir+"\n"), 0o644))

		got, err := paths.FindRepoRoot(wtDir)
		require.NoError(t, err)
		require.Equal(t, wtDir, got)
	})

	t.Run("worktree with relative gitdir", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()

		// Create the real git dir relative to the worktree.
		realGitDir := filepath.Join(tmp, "main-repo", ".git", "worktrees", "wt")
		require.NoError(t, os.MkdirAll(realGitDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(realGitDir, "HEAD"), []byte("ref: refs/heads/feature\n"), 0o644))

		// Create the worktree directory with a .git file using a relative path.
		wtDir := filepath.Join(tmp, "worktree")
		require.NoError(t, os.MkdirAll(wtDir, 0o755))

		relPath, err := filepath.Rel(wtDir, realGitDir)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: "+relPath+"\n"), 0o644))

		got, err := paths.FindRepoRoot(wtDir)
		require.NoError(t, err)
		require.Equal(t, wtDir, got)
	})

	t.Run("malformed git file", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmp, ".git"), []byte("not a valid git file\n"), 0o644))

		_, err := paths.FindRepoRoot(tmp)
		require.ErrorIs(t, err, kclerrors.ErrFileNotFound)
	})

	t.Run("git file with missing HEAD", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()

		// Create a valid gitdir target but without a HEAD file.
		gitDir := filepath.Join(tmp, "empty-gitdir")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))

		require.NoError(t, os.WriteFile(filepath.Join(tmp, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644))

		_, err := paths.FindRepoRoot(tmp)
		require.ErrorIs(t, err, kclerrors.ErrFileNotFound)
	})

	t.Run("no git directory", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()

		_, err := paths.FindRepoRoot(tmp)
		require.ErrorIs(t, err, kclerrors.ErrFileNotFound)
	})
}
