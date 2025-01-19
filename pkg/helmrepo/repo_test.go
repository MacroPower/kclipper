package helmrepo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/pathutil"
)

func TestAddRepo(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.RepoOpts{
		Name: "test-repo",
		URL:  "https://example.com/charts",
	}

	err := manager.Add(repo)
	require.NoError(t, err)
}

func TestGetRepoByName(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.RepoOpts{
		Name: "test-repo",
		URL:  "https://example.com/charts",
	}

	err := manager.Add(repo)
	require.NoError(t, err)

	retrievedRepo, err := manager.GetByName("test-repo")
	require.NoError(t, err)
	require.Equal(t, "test-repo", retrievedRepo.Name)
}

func TestGetRepoByURL(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.RepoOpts{
		Name: "test-repo",
		URL:  "https://example.com/charts",
	}

	err := manager.Add(repo)
	require.NoError(t, err)

	retrievedRepo, err := manager.GetByURL("https://example.com/charts")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/charts", retrievedRepo.URL.String())
}

func TestAddDuplicateRepo(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.RepoOpts{
		Name: "test-repo",
		URL:  "https://example.com/charts",
	}

	err := manager.Add(repo)
	require.NoError(t, err)

	err = manager.Add(repo)
	require.Error(t, err)
}

func TestGetNonExistentRepo(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	_, err := manager.GetByName("non-existent-repo")
	require.Error(t, err)

	_, err = manager.GetByURL("https://non-existent.com/charts")
	require.NoError(t, err)
}

func TestLocalRepo(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	manager := helmrepo.NewManager(helmrepo.WithAllowedPaths(cwd, filepath.Dir(cwd)))

	repo := &helmrepo.RepoOpts{
		Name: "simple-chart",
		URL:  "./testdata",
	}

	err = manager.Add(repo)
	require.NoError(t, err)
}

func TestOutOfBoundsRepo(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	manager := helmrepo.NewManager(helmrepo.WithAllowedPaths(cwd, filepath.Dir(cwd)))

	tcs := map[string]struct {
		repo *helmrepo.RepoOpts
		err  error
	}{
		"repo outside current path": {
			repo: &helmrepo.RepoOpts{
				Name: "test-repo",
				URL:  "../../example",
			},
			err: pathutil.ErrResolvedOutsideRepo,
		},
		"repo at repo root": {
			repo: &helmrepo.RepoOpts{
				Name: "test-repo",
				URL:  "../",
			},
			err: pathutil.ErrResolvedToRepoRoot,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := manager.Add(tc.repo)
			require.Error(t, err)

			assert.ErrorIs(t, err, tc.err)
		})
	}
}
