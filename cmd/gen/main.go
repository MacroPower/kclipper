package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MacroPower/kclipper/pkg/kclhelm"
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

	//nolint:gosec // G304 not relevant for client-side generation.
	fcb, err := os.Create(filepath.Join(modPath, "chart_base.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	pcb := &kclhelm.ChartBase{}
	if err = pcb.GenerateKCL(fcb); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}
	err = fcb.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	//nolint:gosec // G304 not relevant for client-side generation.
	fcc, err := os.Create(filepath.Join(modPath, "chart_config.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	pcc := &kclhelm.ChartConfig{}
	if err = pcc.GenerateKCL(fcc); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}
	err = fcc.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	//nolint:gosec // G304 not relevant for client-side generation.
	fcr, err := os.Create(filepath.Join(modPath, "chart_repo.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	pcr := &kclhelm.ChartRepo{}
	if err = pcr.GenerateKCL(fcr); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}
	err = fcr.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	//nolint:gosec // G304 not relevant for client-side generation.
	fc, err := os.Create(filepath.Join(modPath, "chart.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	pc := &kclhelm.Chart{}
	if err = pc.GenerateKCL(fc); err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}
	err = fc.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	return nil
}
