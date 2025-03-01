package helmutil

import (
	"fmt"
	"log/slog"
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

	logger := slog.With(
		slog.String("cmd", "chart_repo_add"),
		slog.String("repo", repo.Name),
	)

	logger.Info("check init before add")
	if _, err := c.Init(); err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}

	reposFile := filepath.Join(c.BasePath, "repos.k")
	reposSpec := kclutil.SpecPathJoin("repos", repo.GetSnakeCaseName())

	logger.Info("updating repos.k",
		slog.String("spec", reposSpec),
		slog.String("path", reposFile),
	)
	err := c.updateFile(repo.ToAutomation(), reposFile, initialRepoContents, reposSpec)
	if err != nil {
		return fmt.Errorf("failed to update %q: %w", reposFile, err)
	}

	logger.Info("formatting kcl files", slog.String("path", c.BasePath))
	_, err = kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}
