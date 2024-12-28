package helmutil

import (
	"fmt"
	"os"

	"kcl-lang.io/kpm/pkg/downloader"
	"kcl-lang.io/kpm/pkg/opt"
	kclpkg "kcl-lang.io/kpm/pkg/package"

	"github.com/MacroPower/kclx/internal/version"
)

func (c *ChartPkg) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.BasePath
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	chartPkgVersion := version.Version
	if chartPkgVersion == "" {
		chartPkgVersion = "0.1.2"
	}

	exists, err := kclpkg.ModFileExists(path)
	if err != nil {
		return fmt.Errorf("error checking for kcl.mod existence: %w", err)
	}
	if exists {
		// kcl.mod already exists, nothing to do
		return nil
	}

	pkg := kclpkg.NewKclPkg(&opt.InitOptions{
		InitPath: path,
		Name:     "charts",
		Version:  chartPkgVersion,
	})
	pkg.ModFile.Dependencies.Deps.Set("helm", kclpkg.Dependency{
		Name:    "helm",
		Version: chartPkgVersion,
		Source: downloader.Source{
			Oci: &downloader.Oci{
				Reg:  "ghcr.io",
				Repo: "macropower/kclx/helm",
				Tag:  chartPkgVersion,
			},
		},
	})
	if err := pkg.ModFile.StoreModFile(); err != nil {
		return fmt.Errorf("failed to store mod file: %w", err)
	}
	if err := pkg.UpdateModAndLockFile(); err != nil {
		return fmt.Errorf("failed to update kcl.mod: %w", err)
	}

	return nil
}
