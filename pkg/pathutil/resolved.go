// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
// Licensed under the Apache License, Version 2.0

package pathutil

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrMaxNestingLevelReached = errors.New("maximum nesting level reached")
	ErrResolvePath            = errors.New("internal error: failed to resolve path; check logs for more details")
	ErrURLSchemeNotAllowed    = errors.New("the URL scheme is not allowed")
	ErrResolvedOutsideRepo    = errors.New("file resolved to outside repository root")
	ErrResolvedToRepoRoot     = errors.New("path resolved to repository root, which is not allowed")
)

// ResolvedFilePath represents a resolved file path and is intended to prevent unintentional use of an unverified file
// path. It is always either a URL or an absolute path.
type ResolvedFilePath struct {
	path string
	url  *url.URL
}

// URL returns the resolved ([*url.URL], true) if the path is a remote URL,
// otherwise it returns (nil, false).
func (r ResolvedFilePath) URL() (*url.URL, bool) {
	return r.url, r.url != nil
}

// String returns the resolved absolute file path or URL as a string.
func (r ResolvedFilePath) String() string {
	return r.path
}

// ResolvedFileOrDirectoryPath represents a resolved file or directory path and is intended to prevent unintentional use
// of an unverified file or directory path. It is an absolute path.
type ResolvedFileOrDirectoryPath string

// String returns the resolved absolute file or directory path as a string.
func (r ResolvedFileOrDirectoryPath) String() string {
	return string(r)
}

// ResolveSymbolicLinkRecursive resolves the symlink path recursively to its
// canonical path on the file system, with a maximum nesting level of maxDepth.
// If path is not a symlink, returns the verbatim copy of path and err of nil.
func ResolveSymbolicLinkRecursive(path string, maxDepth int) (string, error) {
	resolved, err := os.Readlink(path)
	if err != nil {
		// path is not a symbolic link
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return path, nil
		}
		// Other error has occurred
		return "", fmt.Errorf("failed to read link for path '%s': %w", path, err)
	}

	if maxDepth == 0 {
		return "", ErrMaxNestingLevelReached
	}

	// If we resolved to a relative symlink, make sure we use the absolute
	// path for further resolving
	if !strings.HasPrefix(resolved, string(os.PathSeparator)) {
		basePath := filepath.Dir(path)
		resolved = filepath.Join(basePath, resolved)
	}

	return ResolveSymbolicLinkRecursive(resolved, maxDepth-1)
}

// isURLSchemeAllowed returns true if the protocol scheme is in the list of
// allowed URL schemes.
func isURLSchemeAllowed(scheme string, allowed []string) bool {
	isAllowed := false
	if len(allowed) > 0 {
		for _, s := range allowed {
			if strings.EqualFold(scheme, s) {
				isAllowed = true
				break
			}
		}
	}

	// Empty scheme means local file
	return isAllowed && scheme != ""
}

// We do not provide the path in the error message, because it will be
// returned to the user and could be used for information gathering.
// Instead, we log the concrete error details.
func resolveFailure(path string, err error) error {
	slog.Error("failed to resolve path", "path", path, "err", err)
	return fmt.Errorf("%w: %w", ErrResolvePath, err)
}

func ResolveFileOrDirectoryPath(currentPath, repoRoot, dir string) (ResolvedFileOrDirectoryPath, error) {
	path, err := resolveFileOrDirectory(currentPath, repoRoot, dir, true)
	if err != nil {
		return "", err
	}

	return ResolvedFileOrDirectoryPath(path), nil
}

// ResolveFilePathOrURL will inspect and resolve given file, and make sure
// that its final path is within the boundaries of the path specified in
// repoRoot.
//
// currentPath is the path we're operating in, e.g. where a Helm chart was unpacked
// to. repoRoot is the path to the root of the repository.
//
// If either currentPath or repoRoot is relative, it will be treated as relative
// to the current working directory.
//
// file is the path to a file, relative to currentPath. If file is
// specified as an absolute path (i.e. leading slash), it will be treated as
// relative to the repoRoot. In case file is a symlink in the extracted
// chart, it will be resolved recursively and the decision of whether it is in
// the boundary of repoRoot will be made using the final resolved path.
// file can also be a remote URL with a protocol scheme as prefix,
// in which case the scheme must be included in the list of allowed schemes
// specified by allowedURLSchemes.
//
// Will return an error if either file is outside the boundaries of the
// repoRoot, file is an URL with a forbidden protocol scheme or if
// file is a recursive symlink nested too deep. May return errors for
// other reasons as well.
//
// resolvedPath will hold the absolute, resolved path for file on success
// or set to the empty string on failure.
func ResolveFilePathOrURL(
	currentPath, repoRoot, file string, allowedURLSchemes []string,
) (ResolvedFilePath, error) {
	// A file can be specified as an URL to a remote resource.
	// We only allow certain URL schemes for remote files.
	url, err := url.Parse(file)
	if err == nil {
		// If scheme is empty, it means we parsed a path only
		if url.Scheme != "" {
			if isURLSchemeAllowed(url.Scheme, allowedURLSchemes) {
				return ResolvedFilePath{path: file, url: url}, nil
			}
			return ResolvedFilePath{}, fmt.Errorf("%w: %s", ErrURLSchemeNotAllowed, url.Scheme)
		}
	}

	path, err := resolveFileOrDirectory(currentPath, repoRoot, file, false)
	if err != nil {
		return ResolvedFilePath{}, err
	}

	return ResolvedFilePath{path: path}, nil
}

func resolveFileOrDirectory(
	currentPath string, repoRoot string, fileOrDirectory string, allowResolveToRoot bool,
) (string, error) {
	// Ensure that our repository root is absolute
	absRepoPath, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", resolveFailure(repoRoot, err)
	}

	// If the path to the file or directory is relative, join it with the current working directory (currentPath)
	// Otherwise, join it with the repository's root
	path := fileOrDirectory
	if !filepath.IsAbs(path) {
		absWorkDir, err := filepath.Abs(currentPath)
		if err != nil {
			return "", resolveFailure(repoRoot, err)
		}
		path = filepath.Join(absWorkDir, path)
	} else {
		path = filepath.Join(absRepoPath, path)
	}

	// Ensure any symbolic link is resolved before we evaluate the path
	delinkedPath, err := ResolveSymbolicLinkRecursive(path, 10)
	if err != nil {
		return "", resolveFailure(repoRoot, err)
	}
	path = delinkedPath

	// Resolve the joined path to an absolute path
	path, err = filepath.Abs(path)
	if err != nil {
		return "", resolveFailure(repoRoot, err)
	}

	// Ensure our root path has a trailing slash, otherwise the following check
	// would return true if root is /foo and path would be /foo2
	requiredRootPath := absRepoPath
	if !strings.HasSuffix(requiredRootPath, string(os.PathSeparator)) {
		requiredRootPath += string(os.PathSeparator)
	}

	resolvedToRoot := path+string(os.PathSeparator) == requiredRootPath
	if resolvedToRoot {
		if !allowResolveToRoot {
			return "", fmt.Errorf("%w: %s", ErrResolvedToRepoRoot, path)
		}
	} else {
		// Make sure that the resolved path to file is within the repository's root path
		if !strings.HasPrefix(path, requiredRootPath) {
			return "", fmt.Errorf("%w: %s", ErrResolvedOutsideRepo, fileOrDirectory)
		}
	}

	return path, nil
}
