package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/macropower/kclipper/pkg/kclmodule/kclhelm"
)

func main() {
	basePath := "modules"
	err := generate(basePath)
	if err != nil {
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
	err = pcb.GenerateKCL(fcb)
	if err != nil {
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
	err = pcc.GenerateKCL(fcc)
	if err != nil {
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
	err = pcr.GenerateKCL(fcr)
	if err != nil {
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
	err = pc.GenerateKCL(fc)
	if err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}

	err = fc.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	//nolint:gosec // G304 not relevant for client-side generation.
	fvc, err := os.Create(filepath.Join(modPath, "value_inference_config.k"))
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	pvc := &kclhelm.ValueInferenceConfig{}
	err = pvc.GenerateKCL(fvc)
	if err != nil {
		return fmt.Errorf("failed to generate KCL: %w", err)
	}

	err = fvc.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	return nil
}
