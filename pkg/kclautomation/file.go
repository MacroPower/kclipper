package kclautomation

import (
	"fmt"
	"os"
	"sync"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/kclerrors"
)

// File is a concurrency-safe KCL file writer.
var File = &file{}

type file struct {
	mu sync.Mutex
}

func (f *file) OverrideFile(file string, specs, importPaths []string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !fileExists(file) {
		if err := os.WriteFile(file, []byte(""), 0o600); err != nil {
			return false, fmt.Errorf("%w %q: %w", kclerrors.ErrWriteFile, file, err)
		}
	}

	ok, err := kcl.OverrideFile(file, specs, importPaths)
	if err != nil {
		return ok, fmt.Errorf("%w %q: %w", kclerrors.ErrOverrideFile, file, err)
	}

	return ok, nil
}

func fileExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.IsDir() {
		return false
	}

	return true
}
