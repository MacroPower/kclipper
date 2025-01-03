package helm

import (
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/chartutil"
)

const (
	ResourcePolicyAnnotation = "helm.sh/resource-policy"
	ResourcePolicyKeep       = "keep"
)

type HelmRepository struct {
	Creds
	Name      string
	Repo      string
	EnableOci bool
}

// Helm provides wrapper functionality around the `helm` command.
type Helm interface {
	// Template returns a list of unstructured objects from a `helm template` command
	Template(opts *TemplateOpts) (string, string, error)
	// DependencyBuild runs `helm dependency build` to download a chart's dependencies
	DependencyBuild() error
	// Dispose deletes temp resources
	Dispose()
}

// NewHelmApp create a new wrapper to run commands on the `helm` command-line tool.
func NewHelmApp(workDir string, repos []HelmRepository, isLocal bool, version string, proxy string, noProxy string, passCredentials bool) (Helm, error) {
	cmd, err := NewCmd(workDir, version, proxy, noProxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create new helm command: %w", err)
	}
	cmd.IsLocal = isLocal

	return &helm{repos: repos, cmd: *cmd, passCredentials: passCredentials}, nil
}

type helm struct {
	cmd             Cmd
	repos           []HelmRepository
	passCredentials bool
}

var _ Helm = &helm{}

// IsMissingDependencyErr tests if the error is related to a missing chart dependency.
func IsMissingDependencyErr(err error) bool {
	return strings.Contains(err.Error(), "found in requirements.yaml, but missing in charts") ||
		strings.Contains(err.Error(), "found in Chart.yaml, but missing in charts/ directory")
}

func (h *helm) Template(templateOpts *TemplateOpts) (string, string, error) {
	out, command, err := h.cmd.template(".", templateOpts)
	if err != nil {
		return "", command, fmt.Errorf("failed to execute helm template command: %w", err)
	}
	return out, command, nil
}

func (h *helm) DependencyBuild() error {
	isHelmOci := h.cmd.IsHelmOci
	defer func() {
		h.cmd.IsHelmOci = isHelmOci
	}()

	for i := range h.repos {
		repo := h.repos[i]
		if repo.EnableOci {
			h.cmd.IsHelmOci = true
			if repo.Creds.Username != "" && repo.Creds.Password != "" {
				_, err := h.cmd.RegistryLogin(repo.Repo, repo.Creds)

				defer func() {
					_, _ = h.cmd.RegistryLogout(repo.Repo, repo.Creds)
				}()

				if err != nil {
					return fmt.Errorf("failed to login to registry %s: %w", repo.Repo, err)
				}
			}
		} else {
			_, err := h.cmd.RepoAdd(repo.Name, repo.Repo, repo.Creds, h.passCredentials)
			if err != nil {
				return fmt.Errorf("failed to add helm repository %s: %w", repo.Repo, err)
			}
		}
	}
	h.repos = nil
	_, err := h.cmd.dependencyBuild()
	if err != nil {
		return fmt.Errorf("failed to build helm dependencies: %w", err)
	}
	return nil
}

func (h *helm) Dispose() {
	h.cmd.Close()
}

func Version(shortForm bool) (string, error) {
	hv := chartutil.DefaultCapabilities.HelmVersion
	if shortForm {
		return hv.Version, nil
	}
	return fmt.Sprintf("%#v", hv), nil
}
