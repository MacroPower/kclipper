package helmrepo

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

var DefaultManager = NewManager()

type Repo struct {
	// Helm chart repository name for reference by `@name`.
	Name string `json:"name"`
	URL  string `json:"url"`
	url  *url.URL

	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	CAPath            string
	TLSClientCertData []byte
	TLSClientCertKey  []byte

	InsecureSkipVerify bool
	PassCredentials    bool
}

// IsLocal returns true if the repo URL is a local file path.
func (r *Repo) IsLocal() bool {
	return r.url.Host == ""
}

type Getter interface {
	Get(repo string) (*Repo, error)
}

// Manager manages a collection of [Repo]s.
type Manager struct {
	reposByName map[string]*Repo
	reposByURL  map[string]*Repo

	mu sync.RWMutex
}

// NewManager creates a new [Manager].
func NewManager() *Manager {
	return &Manager{
		reposByName: make(map[string]*Repo),
		reposByURL:  make(map[string]*Repo),
	}
}

// Add adds a new repo to the [Manager]. If a repo with the same name or URL
// already exists, an error is returned.
func (m *Manager) Add(repo *Repo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.reposByName[repo.Name]; ok {
		return fmt.Errorf("repo with name '%s' already exists", repo.Name)
	}

	u, err := url.Parse(repo.URL)
	if err != nil {
		return fmt.Errorf("failed to parse URL '%s': %w", repo.URL, err)
	}
	repoURL := u.String()

	if _, ok := m.reposByURL[repoURL]; ok {
		return fmt.Errorf("repo with URL '%s' already exists", repo.URL)
	}

	repo.url = u
	m.reposByName[repo.Name] = repo
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

// GetByURL returns a repo by its URL. If the repo does not exist in the
// [Manager], a new [Repo] is created with the URL as the name.
func (m *Manager) GetByURL(repoURL string) (*Repo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL '%s': %w", repoURL, err)
	}
	repoURL = u.String()

	repo, ok := m.reposByURL[repoURL]
	if !ok {
		return &Repo{
			Name: repoURL,
			URL:  repoURL,
			url:  u,
		}, nil
	}
	return repo, nil
}
