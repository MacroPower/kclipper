package kclutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrFileNotFound        = errors.New("file not found")
	ErrResolvedOutsideRepo = errors.New("file resolved to outside repository root")
)

// FindTopPkgRoot finds the topmost `kcl.mod` file for the provided path. It is
// similar to [kcl-lang.io/kcl-go/pkg/utils.FindPkgRoot], but climbs as high as
// possible until the provided root directory is reached.
func FindTopPkgRoot(root, path string) (string, error) {
	return findTopFile(root, path, "kcl.mod", false)
}

// FindRepoRoot returns topmost (i.e. passing submodules) git repository for the
// provided path. If no git repository is found, it will return an error.
func FindRepoRoot(path string) (string, error) {
	return findTopFile("/", path, ".git", true)
}

func findTopFile(root, path, target string, isDir bool) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	if !strings.HasPrefix(pathAbs, rootAbs) {
		return "", ErrResolvedOutsideRepo
	}
	pathRel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	currentDir := rootAbs
	for _, part := range strings.Split(pathRel, "/") {
		currentDir = filepath.Join(currentDir, part)
		checkPath := filepath.Join(currentDir, target)
		if fi, err := os.Lstat(checkPath); err == nil && isDir == fi.IsDir() {
			return currentDir, nil
		}
	}

	return "", fmt.Errorf("%s: %w", target, ErrFileNotFound)
}
