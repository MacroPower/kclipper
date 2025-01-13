package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MacroPower/kclipper/pkg/helmmodels/pluginmodule"
)

func main() {
	basePath := "modules"
	if err := generate(basePath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func generate(path string) error {
	modPath := filepath.Join(path, "helm")

	fcb, err := os.Create(filepath.Join(modPath, "chart_base.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fcb.Close()
	pcb := &pluginmodule.ChartBase{}
	if err = pcb.GenerateKCL(fcb); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}

	fcc, err := os.Create(filepath.Join(modPath, "chart_config.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fcc.Close()
	pcc := &pluginmodule.ChartConfig{}
	if err = pcc.GenerateKCL(fcc); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}

	fcr, err := os.Create(filepath.Join(modPath, "chart_repo.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fcr.Close()
	pcr := &pluginmodule.ChartRepo{}
	if err = pcr.GenerateKCL(fcr); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}

	fc, err := os.Create(filepath.Join(modPath, "chart.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fc.Close()
	pc := &pluginmodule.Chart{}
	if err = pc.GenerateKCL(fc); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}

	return nil
}
