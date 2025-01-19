package helmrepo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
)

func TestAddRepo(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.Repo{
		Name: "test-repo",
		URL:  "https://example.com/charts",
	}

	err := manager.Add(repo)
	require.NoError(t, err)
}

func TestGetRepoByName(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.Repo{
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

	repo := &helmrepo.Repo{
		Name: "test-repo",
		URL:  "https://example.com/charts",
	}

	err := manager.Add(repo)
	require.NoError(t, err)

	retrievedRepo, err := manager.GetByURL("https://example.com/charts")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/charts", retrievedRepo.URL)
}

func TestAddDuplicateRepo(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.Repo{
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
