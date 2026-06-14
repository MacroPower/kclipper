package kclautomation

import (
	"fmt"
	"os"
	"sync"

	"kcl-lang.io/kcl-go"

	"github.com/macropower/kclipper/internal/osutil"
	"github.com/macropower/kclipper/pkg/kclerrors"
)

// File is a concurrency-safe KCL file writer.
var File = &file{}

type file struct {
	mu sync.Mutex
}

func (f *file) OverrideFile(file string, specs, importPaths []string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !osutil.FileExists(file) {
		err := os.WriteFile(file, []byte(""), 0o600)
		if err != nil {
			return false, fmt.Errorf("%w %q: %w", kclerrors.ErrWriteFile, file, err)
		}
	}

	ok, err := kcl.OverrideFile(file, specs, importPaths)
	if err != nil {
		return ok, fmt.Errorf("%w %q: %w", kclerrors.ErrOverrideFile, file, err)
	}

	return ok, nil
}
