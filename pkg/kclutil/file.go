package kclutil

import (
	"fmt"
	"os"
	"sync"

	"kcl-lang.io/kcl-go"

	kclutil "kcl-lang.io/kcl-go/pkg/utils"
)

// File is a concurrency-safe KCL file writer.
var File = &file{}

type file struct {
	mu sync.Mutex
}

func (f *file) OverrideFile(file string, specs, importPaths []string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !kclutil.FileExists(file) {
		if err := os.WriteFile(file, []byte(""), 0o600); err != nil {
			return false, fmt.Errorf("failed to write %q: %w", file, err)
		}
	}

	ok, err := kcl.OverrideFile(file, specs, importPaths)
	if err != nil {
		return ok, fmt.Errorf("failed to override %q: %w", file, err)
	}

	return ok, nil
}
