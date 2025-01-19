package helmutil

import (
	"fmt"
	"os"
	"path"
	"sort"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/kclhelm"
)

const initialRepoContents = `import helm

repos: helm.ChartRepos = {}
`

func (c *ChartPkg) AddRepo(repo *kclhelm.ChartRepo) error {
	if err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}

	repoConfig := map[string]stringOrBool{
		"name":                  newString(repo.Name),
		"url":                   newString(repo.URL),
		"usernameEnv":           newString(repo.UsernameEnv),
		"passwordEnv":           newString(repo.PasswordEnv),
		"caPath":                newString(repo.CAPath),
		"tlsClientCertDataPath": newString(repo.TLSClientCertDataPath),
		"tlsClientCertKeyPath":  newString(repo.TLSClientCertKeyPath),
		"insecureSkipVerify":    newBool(repo.InsecureSkipVerify),
		"passCredentials":       newBool(repo.PassCredentials),
	}
	if err := c.updateReposFile(c.BasePath, repo.GetSnakeCaseName(), repoConfig); err != nil {
		return err
	}

	_, err := kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}

func (c *ChartPkg) updateReposFile(vendorDir, repoKey string, repoConfig map[string]stringOrBool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	reposFile := path.Join(vendorDir, "repos.k")
	if !fileExists(reposFile) {
		if err := os.WriteFile(reposFile, []byte(initialRepoContents), 0o600); err != nil {
			return fmt.Errorf("failed to write '%s': %w", reposFile, err)
		}
	}
	imports := []string{"helm"}
	specs := sort.StringSlice{}
	for k, v := range repoConfig {
		if k == "" {
			return fmt.Errorf("invalid key in repo config: %#v", repoConfig)
		}
		if !v.IsSet() {
			continue
		}
		if v.IsString() {
			specs = append(specs, fmt.Sprintf(`repos.%s.%s="%s"`, repoKey, k, v.s))
		} else {
			specs = append(specs, fmt.Sprintf(`repos.%s.%s=True`, repoKey, k))
		}
	}
	specs.Sort()
	_, err := kcl.OverrideFile(reposFile, specs, imports)
	if err != nil {
		return fmt.Errorf("failed to update '%s': %w", reposFile, err)
	}
	return nil
}

type stringOrBool struct {
	s string
	b bool
}

func (s stringOrBool) IsString() bool {
	return s.s != ""
}

func (s stringOrBool) IsSet() bool {
	return s.s != "" || s.b
}

func newString(s string) stringOrBool {
	return stringOrBool{s: s}
}

func newBool(b bool) stringOrBool {
	return stringOrBool{b: b}
}
