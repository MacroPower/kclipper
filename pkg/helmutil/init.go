package helmutil

import (
	"errors"
	"fmt"
	"os"

	"github.com/MacroPower/kclx/internal/version"
	"kcl-lang.io/kpm/pkg/downloader"
	"kcl-lang.io/kpm/pkg/opt"
	kclpkg "kcl-lang.io/kpm/pkg/package"
)

func ChartInit(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
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
		return errors.New("kcl.mod already exists")
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
