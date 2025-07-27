package chartcmd

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"kcl-lang.io/kcl-go"

	"github.com/macropower/kclipper/pkg/kclautomation"
	"github.com/macropower/kclipper/pkg/kclmodule/kclhelm"
)

const initialRepoContents = `import helm

repos: helm.ChartRepos = {}
`

func (c *KCLPackage) AddRepo(repo *kclhelm.ChartRepo) error {
	err := repo.Validate()
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger := slog.With(
		slog.String("cmd", "chart_repo_add"),
		slog.String("repo", repo.Name),
	)

	logger.Info("check init before add")

	_, err = c.Init()
	if err != nil {
		return fmt.Errorf("failed to init before add: %w", err)
	}

	reposFile := filepath.Join(c.BasePath, "repos.k")
	reposSpec := kclautomation.SpecPathJoin("repos", repo.GetSnakeCaseName())

	logger.Info("updating repos.k",
		slog.String("spec", reposSpec),
		slog.String("path", reposFile),
	)

	err = c.updateFile(repo.ToAutomation(), reposFile, initialRepoContents, reposSpec)
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
