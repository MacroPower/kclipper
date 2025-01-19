package helmrepo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/MacroPower/kclipper/pkg/pathutil"
)

var DefaultManager = NewManager()

var (
	ErrRepoNameEmpty       = errors.New("repo name cannot be empty")
	ErrRepoURLEmpty        = errors.New("repo URL cannot be empty")
	ErrFailedToResolveURL  = errors.New("failed to resolve URL")
	ErrFailedToResolveFile = errors.New("failed to resolve file path")
)

type Repo struct {
	// Helm chart repository name for reference by `@name`.
	Name                  string
	URL                   pathutil.ResolvedFilePath
	Username              string
	Password              string
	CAPath                pathutil.ResolvedFileOrDirectoryPath
	TLSClientCertDataPath pathutil.ResolvedFileOrDirectoryPath
	TLSClientCertKeyPath  pathutil.ResolvedFileOrDirectoryPath
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
	reposByName map[string]*Repo
	reposByURL  map[string]*Repo

	allowedURLSchemes     []string
	currentPath, repoRoot string

	mu sync.RWMutex
}

// NewManager creates a new [Manager].
func NewManager(opt ...ManagerOpt) *Manager {
	m := &Manager{
		reposByName:       make(map[string]*Repo),
		reposByURL:        make(map[string]*Repo),
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
	if err := repoOpts.Validate(); err != nil {
		return nil, err
	}

	repo := &Repo{
		Name:               repoOpts.Name,
		Username:           repoOpts.Username,
		Password:           repoOpts.Password,
		InsecureSkipVerify: repoOpts.InsecureSkipVerify,
		PassCredentials:    repoOpts.PassCredentials,
	}

	p, err := pathutil.ResolveFilePathOrURL(m.currentPath, m.repoRoot, repoOpts.URL, m.allowedURLSchemes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToResolveURL, err)
	}
	repo.URL = p

	if repoOpts.CAPath != "" {
		p, err := pathutil.ResolveFileOrDirectoryPath(m.currentPath, m.repoRoot, repoOpts.CAPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToResolveFile, err)
		}
		repo.CAPath = p
	}
	if repoOpts.TLSClientCertDataPath != "" {
		p, err := pathutil.ResolveFileOrDirectoryPath(m.currentPath, m.repoRoot, repoOpts.TLSClientCertDataPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFailedToResolveFile, err)
		}
		repo.TLSClientCertDataPath = p
	}
	if repoOpts.TLSClientCertKeyPath != "" {
		p, err := pathutil.ResolveFileOrDirectoryPath(m.currentPath, m.repoRoot, repoOpts.TLSClientCertKeyPath)
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
	repoName := repo.Name
	repoURL := repo.URL.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.reposByName[repoName]; ok {
		return fmt.Errorf("repo with name '%s' already exists", repoName)
	}
	if _, ok := m.reposByURL[repoURL]; ok {
		return fmt.Errorf("repo with URL '%s' already exists", repoURL)
	}

	m.reposByName[repoName] = repo
	m.reposByURL[repoURL] = repo

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
	m.mu.RLock()
	defer m.mu.RUnlock()

	repo, ok := m.reposByName[name]
	if !ok {
		return nil, fmt.Errorf("repo with name '%s' not found", name)
	}
	return repo, nil
}

// GetByURL returns a [Repo] by its URL. If the [Repo] does not exist in the
// [Manager], a new [Repo] is created with the URL as the name.
func (m *Manager) GetByURL(repoURL string) (*Repo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL '%s': %w", repoURL, err)
	}
	repoURL = u.String()

	if repo, ok := m.reposByURL[repoURL]; ok {
		return repo, nil
	}

	p, err := pathutil.ResolveFilePathOrURL(m.currentPath, m.repoRoot, repoURL, m.allowedURLSchemes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToResolveURL, err)
	}
	return &Repo{
		Name: repoURL,
		URL:  p,
	}, nil
}
