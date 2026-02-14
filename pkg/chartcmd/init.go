package chartcmd

import (
	"fmt"
	"log/slog"
	"os"

	"go.jacobcolvin.com/x/version"
	"kcl-lang.io/kpm/pkg/downloader"
	"kcl-lang.io/kpm/pkg/opt"

	kclpkg "kcl-lang.io/kpm/pkg/package"
)

func (c *KCLPackage) Init() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.BasePath

	logger := slog.With(
		slog.String("cmd", "init"),
		slog.String("path", path),
	)

	logger.Debug("ensure package directory")

	err := os.MkdirAll(path, 0o750)
	if err != nil {
		return false, fmt.Errorf("create charts directory: %w", err)
	}

	logger.Debug("ensured package directory")

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
		return false, fmt.Errorf("check kcl.mod existence: %w", err)
	}

	if exists {
		logger.Debug("kcl.mod already exists, nothing to do")

		return false, nil
	}

	logger.Info("creating new kcl.mod file")

	pkg := kclpkg.NewKclPkg(&opt.InitOptions{
		InitPath: path,
		Name:     "charts",
		Version:  chartPkgVersion,
	})
	pkg.ModFile.Deps.Set("helm", kclpkg.Dependency{
		Name:    "helm",
		Version: chartPkgVersion,
		Source:  source,
	})

	err = pkg.ModFile.StoreModFile()
	if err != nil {
		return false, fmt.Errorf("store mod file: %w", err)
	}

	logger.Info("updating kcl.mod and kcl.mod.lock")

	err = pkg.UpdateModAndLockFile()
	if err != nil {
		return false, fmt.Errorf("update kcl.mod: %w", err)
	}

	return true, nil
}
