package helmutil

import (
	"fmt"
	"os"

	"kcl-lang.io/kpm/pkg/downloader"
	"kcl-lang.io/kpm/pkg/opt"

	kclpkg "kcl-lang.io/kpm/pkg/package"

	"github.com/MacroPower/kclipper/internal/version"
)

func (c *ChartPkg) Init() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.BasePath
	if err := os.MkdirAll(path, 0o750); err != nil {
		return false, fmt.Errorf("failed to create charts directory: %w", err)
	}

	source := downloader.Source{
		Local: &downloader.Local{
			Path: "../modules/helm",
		},
	}

	chartPkgVersion := version.Version
	if chartPkgVersion != "" {
		source = downloader.Source{
			Oci: &downloader.Oci{
				Reg:  "ghcr.io",
				Repo: "macropower/kclipper/helm",
				Tag:  chartPkgVersion,
			},
		}
	}

	exists, err := kclpkg.ModFileExists(path)
	if err != nil {
		return false, fmt.Errorf("error checking for kcl.mod existence: %w", err)
	}

	if exists {
		// `kcl.mod` already exists, nothing to do.
		return false, nil
	}

	pkg := kclpkg.NewKclPkg(&opt.InitOptions{
		InitPath: path,
		Name:     "charts",
		Version:  chartPkgVersion,
	})
	pkg.ModFile.Dependencies.Deps.Set("helm", kclpkg.Dependency{
		Name:    "helm",
		Version: chartPkgVersion,
		Source:  source,
	})

	if err := pkg.ModFile.StoreModFile(); err != nil {
		return false, fmt.Errorf("failed to store mod file: %w", err)
	}

	if err := pkg.UpdateModAndLockFile(); err != nil {
		return false, fmt.Errorf("failed to update kcl.mod: %w", err)
	}

	return true, nil
}
