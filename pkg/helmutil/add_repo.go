package helmutil

import (
	"fmt"
	"path/filepath"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/kclhelm"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

const initialRepoContents = `import helm

repos: helm.ChartRepos = {}
`

func (c *ChartPkg) AddRepo(repo *kclhelm.ChartRepo) error {
	if err := repo.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	if _, err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}

	reposFile := filepath.Join(c.BasePath, "repos.k")
	reposSpec := kclutil.SpecPathJoin("repos", repo.GetSnakeCaseName())

	err := c.updateFile(repo.ToAutomation(), reposFile, initialRepoContents, reposSpec)
	if err != nil {
		return fmt.Errorf("failed to update %q: %w", reposFile, err)
	}

	_, err = kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}
