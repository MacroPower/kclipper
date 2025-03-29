package kclutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindTopPkgRoot finds the topmost `kcl.mod` file for the provided path. It is
// similar to both [kcl-lang.io/kcl-go/pkg/utils.FindPkgRoot] and
// [kcl-lang.io/kcl-go/pkg/tools/list.FindPkgInfo], but adds additional guards
// to prevent resolving outside the provided root. It also searches downward
// from the provided root and returns the first match, rather than searching
// upward from the provided path until the filesystem root.
func FindTopPkgRoot(root, path string) (string, error) {
	target := "kcl.mod"

	f, err := findTopFile(root, path, func(s string) (bool, error) {
		checkPath := filepath.Join(s, "kcl.mod")
		fi, err := os.Lstat(checkPath)
		if err != nil {
			return false, fmt.Errorf("%s: %w", checkPath, err)
		}
		if fi.IsDir() {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", target, err)
	}

	return f, nil
}

// FindRepoRoot returns topmost (i.e. passing submodules) git repository for the
// provided path. If no git repository is found, it will return an error.
func FindRepoRoot(path string) (string, error) {
	// Ideally this would be `git rev-parse --show-toplevel` but I didn't want to
	// add another package just for this. To see what is normally looked up:
	// 	sudo ktrace trace -S -f C3 -c git rev-parse --show-toplevel | grep .git/

	// Look for a `.git` directory containing a `HEAD` file.
	target1 := ".git"
	target2 := "HEAD"

	f, err := findTopFile("/", path, func(s string) (bool, error) {
		checkPath1 := filepath.Join(s, target1)
		fi1, err := os.Lstat(checkPath1)
		if err != nil {
			return false, fmt.Errorf("%s: %w", checkPath1, err)
		}
		if !fi1.IsDir() {
			return false, nil
		}
		checkPath2 := filepath.Join(s, target1, target2)
		fi2, err := os.Lstat(checkPath2)
		if err != nil {
			return false, fmt.Errorf("%s: %w", checkPath2, err)
		}
		if fi2.IsDir() {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", filepath.Join(target1, target2), err)
	}

	return f, nil
}

func findTopFile(root, path string, test func(string) (bool, error)) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %w", err)
	}

	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %w", err)
	}
	if !strings.HasPrefix(pathAbs, rootAbs) {
		return "", ErrResolvedOutsideRepo
	}

	pathRel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return "", fmt.Errorf("get relative path: %w", err)
	}

	currentDir := rootAbs
	for part := range strings.SplitSeq(pathRel, "/") {
		currentDir = filepath.Join(currentDir, part)
		if match, err := test(currentDir); err == nil && match {
			return currentDir, nil
		}
	}

	return "", ErrFileNotFound
}
