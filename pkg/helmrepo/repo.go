package helmrepo

import (
	"fmt"
	"strings"
)

var DefaultManager = NewManager()

type Repo struct {
	// Helm chart repository name for reference by `@name`.
	Name string `json:"name"`
	URL  string `json:"url"`

	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	CAPath            string
	TLSClientCertData string
	TLSClientCertKey  string

	InsecureSkipVerify bool
	PassCredentials    bool
}

type Manager struct {
	reposByName map[string]*Repo
	reposByURL  map[string]*Repo
}

func NewManager() *Manager {
	return &Manager{
		reposByName: make(map[string]*Repo),
		reposByURL:  make(map[string]*Repo),
	}
}

func (m *Manager) Add(repo *Repo) error {
	if _, ok := m.reposByName[repo.Name]; ok {
		return fmt.Errorf("repo with name %s already exists", repo.Name)
	}

	if _, ok := m.reposByURL[repo.URL]; ok {
		return fmt.Errorf("repo with URL %s already exists", repo.URL)
	}

	m.reposByName[repo.Name] = repo
	m.reposByURL[repo.URL] = repo

	return nil
}

func (m *Manager) Get(repo string) (*Repo, error) {
	if strings.HasPrefix(repo, "@") {
		return m.GetByName(strings.TrimPrefix(repo, "@"))
	}

	return m.GetByURL(repo)
}

func (m *Manager) GetByName(name string) (*Repo, error) {
	repo, ok := m.reposByName[name]
	if !ok {
		return nil, fmt.Errorf("repo with name %s not found", name)
	}
	return repo, nil
}

func (m *Manager) GetByURL(url string) (*Repo, error) {
	repo, ok := m.reposByURL[url]
	if !ok {
		return nil, fmt.Errorf("repo with URL %s not found", url)
	}
	return repo, nil
}
