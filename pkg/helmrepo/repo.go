package helmrepo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/MacroPower/kclipper/pkg/paths"
)

var (
	DefaultManager = NewManager()

	ErrRepoNameEmpty       = errors.New("repo name cannot be empty")
	ErrRepoURLEmpty        = errors.New("repo URL cannot be empty")
	ErrFailedToResolveURL  = errors.New("failed to resolve URL")
	ErrFailedToResolveFile = errors.New("failed to resolve file path")
	ErrInvalidRepoURL      = errors.New("invalid repository URL")
)

type DuplicateRepoError struct {
	Name string
}

func (err DuplicateRepoError) Error() string {
	return fmt.Sprintf("repo with name %q already exists", err.Name)
}

type RepoNotFoundError struct {
	Name string
}

func (err RepoNotFoundError) Error() string {
	return fmt.Sprintf("repo with name %q not found", err.Name)
}

type Repo struct {
	// Helm chart repository name for reference by `@name`.
	Name                  string
	URL                   paths.ResolvedFilePath
	Username              string
	Password              string
	CAPath                paths.ResolvedFileOrDirectoryPath
	TLSClientCertDataPath paths.ResolvedFileOrDirectoryPath
	TLSClientCertKeyPath  paths.ResolvedFileOrDirectoryPath
	InsecureSkipVerify    bool
	PassCredentials       bool
}

type RepoOpts struct {
	// Helm chart repository name for reference by `@name`.
	Name                  string `json:"name"`
	URL                   string `json:"url"`
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
	CAPath                string `json:"caPath,omitempty"`
	TLSClientCertDataPath string `json:"tlsClientCertDataPath,omitempty"`
	TLSClientCertKeyPath  string `json:"tlsClientCertKeyPath,omitempty"`
	InsecureSkipVerify    bool   `json:"insecureSkipVerify"`
	PassCredentials       bool   `json:"passCredentials"`
}

// IsLocal returns true if the repo URL is a local file path.
func (r *Repo) IsLocal() bool {
	_, ok := r.URL.URL()

	return !ok
}

func (r *RepoOpts) Validate() error {
	if r.Name == "" {
		return ErrRepoNameEmpty
	}

	if r.URL == "" {
		return ErrRepoURLEmpty
	}

	return nil
}

type Getter interface {
	Get(repo string) (*Repo, error)
}

// Manager manages a collection of [Repo]s.
type Manager struct {
	reposByName       sync.Map
	currentPath       string
	repoRoot          string
	allowedURLSchemes []string
}

// NewManager creates a new [Manager].
func NewManager(opt ...ManagerOpt) *Manager {
	m := &Manager{
		reposByName:       sync.Map{},
		allowedURLSchemes: []string{"http", "https", "oci"},
		currentPath:       ".",
		repoRoot:          ".",
	}
	for _, o := range opt {
		o(m)
	}

	return m
}

// ManagerOpt is a functional option for [Manager].
type ManagerOpt func(*Manager)

// WithAllowedURLSchemes sets the allowed URL schemes for the [Manager].
func WithAllowedURLSchemes(schemes ...string) ManagerOpt {
	return func(m *Manager) {
		m.allowedURLSchemes = schemes
	}
}

func WithAllowedPaths(currentPath, repoRoot string) ManagerOpt {
	return func(m *Manager) {
		m.currentPath = currentPath
		m.repoRoot = repoRoot
	}
}

func (m *Manager) resolveRepo(repoOpts *RepoOpts) (*Repo, error) {
	err := repoOpts.Validate()
	if err != nil {
		return nil, err
	}

	repo := &Repo{
		Name:               repoOpts.Name,
		Username:           repoOpts.Username,
		Password:           repoOpts.Password,
		InsecureSkipVerify: repoOpts.InsecureSkipVerify,
		PassCredentials:    repoOpts.PassCredentials,
	}

	p, err := paths.ResolveFilePathOrURL(m.currentPath, m.repoRoot, repoOpts.URL, m.allowedURLSchemes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToResolveURL, err)
	}

	repo.URL = p

	if repoOpts.CAPath != "" {
		p, err := paths.ResolveFileOrDirectoryPath(m.currentPath, m.repoRoot, repoOpts.CAPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToResolveFile, err)
		}

		repo.CAPath = p
	}

	if repoOpts.TLSClientCertDataPath != "" {
		p, err := paths.ResolveFileOrDirectoryPath(m.currentPath, m.repoRoot, repoOpts.TLSClientCertDataPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToResolveFile, err)
		}

		repo.TLSClientCertDataPath = p
	}

	if repoOpts.TLSClientCertKeyPath != "" {
		p, err := paths.ResolveFileOrDirectoryPath(m.currentPath, m.repoRoot, repoOpts.TLSClientCertKeyPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToResolveFile, err)
		}

		repo.TLSClientCertKeyPath = p
	}

	return repo, nil
}

// Add uses [RepoOpts] to create and add a new [Repo] to the [Manager].
// An error is returned if the [Repo] could not be generated, or if a [Repo]
// with the same Name and/or URL already exists.
func (m *Manager) Add(repoOpts *RepoOpts) error {
	repo, err := m.resolveRepo(repoOpts)
	if err != nil {
		return err
	}

	return m.addByName(repo.Name, repo)
}

func (m *Manager) addByName(name string, repo *Repo) error {
	if _, ok := m.reposByName.Load(name); ok {
		return DuplicateRepoError{Name: name}
	}

	m.reposByName.Store(name, repo)

	return nil
}

// Get returns a repo by its name or URL. It calls [Manager.GetByName] or
// [Manager.GetByURL] depending on the input.
func (m *Manager) Get(repo string) (*Repo, error) {
	if strings.HasPrefix(repo, "@") {
		return m.GetByName(strings.TrimPrefix(repo, "@"))
	}

	return m.GetByURL(repo)
}

// GetByName returns a repo by its name. If the repo does not exist in the
// [Manager], an error is returned.
func (m *Manager) GetByName(name string) (*Repo, error) {
	if repo, ok := m.reposByName.Load(name); ok {
		rr, ok := repo.(*Repo)
		if !ok {
			panic(fmt.Sprintf("repo with name %q is not a *Repo", name))
		}

		return rr, nil
	}

	return nil, RepoNotFoundError{Name: name}
}

// GetByURL returns a [Repo] by its URL. If the [Repo] does not exist in the
// [Manager], a new [Repo] is created with the URL as the name.
func (m *Manager) GetByURL(repoURL string) (*Repo, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("%q: %w: %w", repoURL, ErrInvalidRepoURL, err)
	}

	repoURL = u.String()

	repo, err := m.GetByName(repoURL)
	if err == nil {
		return repo, nil
	}

	p, err := paths.ResolveFilePathOrURL(m.currentPath, m.repoRoot, repoURL, m.allowedURLSchemes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToResolveURL, err)
	}

	return &Repo{
		Name: repoURL,
		URL:  p,
	}, nil
}
