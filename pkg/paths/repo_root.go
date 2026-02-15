package paths

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/macropower/kclipper/pkg/kclerrors"
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

// FindRepoRoot returns the closest (innermost) git repository root for the
// provided path by searching bottom-up from path toward /. This matches the
// behavior of git rev-parse --show-toplevel, correctly resolving worktrees
// nested inside a parent repository. If no git repository is found, it will
// return an error.
func FindRepoRoot(path string) (string, error) {
	// Ideally this would be `git rev-parse --show-toplevel` but I didn't want to
	// add another package just for this. To see what is normally looked up:
	// 	sudo ktrace trace -S -f C3 -c git rev-parse --show-toplevel | grep .git/

	// Look for a `.git` directory containing a `HEAD` file.
	target1 := ".git"
	target2 := "HEAD"

	f, err := findClosestFile("/", path, func(s string) (bool, error) {
		checkPath1 := filepath.Join(s, target1)
		fi1, err := os.Lstat(checkPath1)
		if err != nil {
			return false, fmt.Errorf("%s: %w", checkPath1, err)
		}

		var headPath string

		switch {
		case fi1.IsDir():
			headPath = filepath.Join(checkPath1, target2)
		default:
			gitDir, gitFileErr := resolveGitFile(checkPath1, s)
			if gitFileErr != nil {
				return false, nil //nolint:nilerr // Intentionally skip malformed .git files.
			}

			headPath = filepath.Join(gitDir, target2)
		}

		fi2, err := os.Lstat(headPath)
		if err != nil {
			return false, fmt.Errorf("%s: %w", headPath, err)
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

// resolveGitFile reads a `.git` file (as used in git worktrees) and resolves
// the gitdir path it points to. The file is expected to contain a single line
// in the format `gitdir: <path>`. Relative paths are resolved against baseDir.
func resolveGitFile(dotGitPath, baseDir string) (string, error) {
	f, err := os.Open(dotGitPath) //nolint:gosec // dotGitPath is constructed from filepath.Join, not user input.
	if err != nil {
		return "", fmt.Errorf("open git file: %w", err)
	}
	defer f.Close() //nolint:errcheck // Best-effort close.

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return "", errors.New("empty git file")
	}

	line := strings.TrimSpace(scanner.Text())

	gitDir, found := strings.CutPrefix(line, "gitdir: ")
	if !found {
		return "", errors.New("missing gitdir prefix")
	}

	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(baseDir, gitDir)
	}

	return filepath.Clean(gitDir), nil
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
		match, err := test(currentDir)
		if err == nil && match {
			return currentDir, nil
		}
	}

	return "", kclerrors.ErrFileNotFound
}

// findClosestFile walks from path upward toward root, returning the first
// directory where test returns true. It is the bottom-up counterpart of
// [findTopFile].
func findClosestFile(root, path string, test func(string) (bool, error)) (string, error) {
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

	currentDir := pathAbs
	for {
		match, err := test(currentDir)
		if err == nil && match {
			return currentDir, nil
		}

		if currentDir == rootAbs {
			break
		}

		currentDir = filepath.Dir(currentDir)
	}

	return "", kclerrors.ErrFileNotFound
}
