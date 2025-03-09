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

	retrievedRepo, err := manager.Get("test-repo")
	require.NoError(t, err)
	assert.Equal(t, "test-repo", retrievedRepo.Name)
	assert.False(t, retrievedRepo.IsLocal())
}

func TestGetRepoByURL(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	err := manager.Add(&helmrepo.RepoOpts{
		Name:     "test-repo",
		URL:      "https://example.com/charts",
		Username: "user",
		Password: "pass",
	})
	require.NoError(t, err)

	retrievedRepo, err := manager.Get("https://example.com/charts")
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/charts", retrievedRepo.URL.String())
	assert.Empty(t, retrievedRepo.Username, "URL should not match named repo")
	assert.Empty(t, retrievedRepo.Password, "URL should not match named repo")
}

func TestGetNonExistentRepo(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	_, err := manager.Get("@non-existent-repo")
	require.Error(t, err)
	require.ErrorAs(t, err, &helmrepo.RepoNotFoundError{})
	require.ErrorContains(t, err, "\"non-existent-repo\"")
}

func TestGetInvalidURL(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	manager := helmrepo.NewManager(
		helmrepo.WithAllowedPaths(cwd, filepath.Dir(cwd)),
	)

	tcs := map[string]struct {
		err   error
		query string
	}{
		"out of bounds": {
			query: "../../example",
			err:   pathutil.ErrResolvedOutsideRepo,
		},
		"invalid schema": {
			query: "''://example.com/charts",
			err:   helmrepo.ErrInvalidRepoURL,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := manager.Get(tc.query)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.err)
		})
	}
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

	retrievedRepo, err := manager.Get("@simple-chart")
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(cwd, "testdata"), retrievedRepo.URL.String())
	assert.True(t, retrievedRepo.IsLocal())
}

func TestOCIRepo(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	repo := &helmrepo.RepoOpts{
		Name: "grafana",
		URL:  "oci://ghcr.io/grafana/helm-charts/grafana-operator",
	}

	err := manager.Add(repo)
	require.NoError(t, err)

	retrievedRepo, err := manager.Get("@grafana")
	require.NoError(t, err)

	assert.Equal(t, "oci://ghcr.io/grafana/helm-charts/grafana-operator", retrievedRepo.URL.String())
	assert.False(t, retrievedRepo.IsLocal())
}

func TestInvalidRepo(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	manager := helmrepo.NewManager(
		helmrepo.WithAllowedPaths(cwd, filepath.Dir(cwd)),
		helmrepo.WithAllowedURLSchemes("http", "https"),
	)

	tcs := map[string]struct {
		repo *helmrepo.RepoOpts
		err  error
	}{
		"empty name": {
			repo: &helmrepo.RepoOpts{
				Name: "",
				URL:  "https://example.com/charts",
			},
			err: helmrepo.ErrRepoNameEmpty,
		},
		"empty URL": {
			repo: &helmrepo.RepoOpts{
				Name: "empty-url",
				URL:  "",
			},
			err: helmrepo.ErrRepoURLEmpty,
		},
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
		"invalid scheme": {
			repo: &helmrepo.RepoOpts{
				Name: "invalid-scheme",
				URL:  "invalid://example.com/charts",
			},
			err: helmrepo.ErrFailedToResolveURL,
		},
		"invalid CA cert": {
			repo: &helmrepo.RepoOpts{
				Name:   "invalid-ca-cert",
				URL:    "https://example.com/charts",
				CAPath: "../../example",
			},
			err: pathutil.ErrResolvedOutsideRepo,
		},
		"invalid client cert key": {
			repo: &helmrepo.RepoOpts{
				Name:                 "invalid-client-cert",
				URL:                  "https://example.com/charts",
				TLSClientCertKeyPath: "../../example",
			},
			err: pathutil.ErrResolvedOutsideRepo,
		},
		"invalid client cert data": {
			repo: &helmrepo.RepoOpts{
				Name:                  "invalid-client-cert",
				URL:                   "https://example.com/charts",
				TLSClientCertDataPath: "../../example",
			},
			err: pathutil.ErrResolvedOutsideRepo,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := manager.Add(tc.repo)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func TestAllOpts(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	manager := helmrepo.NewManager()

	repo := &helmrepo.RepoOpts{
		Name:                  "auth-repo",
		URL:                   "https://example.com/charts",
		Username:              "user",
		Password:              "pass",
		CAPath:                "ca.pem",
		TLSClientCertDataPath: "client.pem",
		TLSClientCertKeyPath:  "client.key",
		InsecureSkipVerify:    true,
		PassCredentials:       true,
	}

	err = manager.Add(repo)
	require.NoError(t, err)

	retrievedRepo, err := manager.Get("auth-repo")
	require.NoError(t, err)

	// Verify options are set correctly
	assert.Equal(t, "user", retrievedRepo.Username)
	assert.Equal(t, "pass", retrievedRepo.Password)
	assert.Equal(t, filepath.Join(cwd, "ca.pem"), retrievedRepo.CAPath.String())
	assert.Equal(t, filepath.Join(cwd, "client.pem"), retrievedRepo.TLSClientCertDataPath.String())
	assert.Equal(t, filepath.Join(cwd, "client.key"), retrievedRepo.TLSClientCertKeyPath.String())
	assert.True(t, retrievedRepo.InsecureSkipVerify)
	assert.True(t, retrievedRepo.PassCredentials)
}

func TestMultipleReposWithSameName(t *testing.T) {
	t.Parallel()

	manager := helmrepo.NewManager()

	// Add first repo
	repo1 := &helmrepo.RepoOpts{
		Name: "repo",
		URL:  "https://example.com/1/charts",
	}

	err := manager.Add(repo1)
	require.NoError(t, err)

	// Add second repo with same URL but different name
	repo2 := &helmrepo.RepoOpts{
		Name: "repo",
		URL:  "https://example.com/2/charts",
	}

	err = manager.Add(repo2)
	require.Error(t, err)
	require.ErrorAs(t, err, &helmrepo.DuplicateRepoError{})
	require.ErrorContains(t, err, "\"repo\"")
}
