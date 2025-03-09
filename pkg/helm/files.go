package helm

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/klauspost/compress/gzip"
)

var (
	ErrFailedFileRead  = errors.New("failed to read file")
	ErrFailedFileWrite = errors.New("failed to write file")
	ErrFailedFileClose = errors.New("failed to close file")
	ErrIteratingTar    = errors.New("error iterating on tar reader")
)

type LimitReaderUnexpectedEOFError struct {
	MaxSize int64
}

func (l LimitReaderUnexpectedEOFError) Error() string {
	return fmt.Sprintf(
		"unexpected EOF, the extracted content was likely greater than your defined limit of %d bytes", l.MaxSize,
	)
}

// gunzip will loop over the tar reader creating the file structure at dstPath.
// Callers must make sure dstPath is:
//   - a full path
//   - points to an empty directory or
//   - points to a non existing directory
func gunzip(dstPath string, r io.Reader, maxSize int64, preserveFileMode bool) error {
	if !filepath.IsAbs(dstPath) {
		return fmt.Errorf("dstPath points to a relative path: %s", dstPath)
	}

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedFileRead, err)
	}
	defer func() {
		err = gzr.Close()
		if err != nil {
			slog.Error("failed to close gzip reader",
				slog.Any("err", err),
			)
		}
	}()

	var tr *tar.Reader

	if maxSize != 0 {
		lr := io.LimitReader(gzr, maxSize)
		tr = tar.NewReader(lr)
	} else {
		tr = tar.NewReader(gzr)
	}

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			if maxSize != 0 && errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("%w: %w", ErrIteratingTar, LimitReaderUnexpectedEOFError{maxSize})
			}

			return fmt.Errorf("%w: %w", ErrIteratingTar, err)
		}

		if header == nil || header.Name == "." {
			continue
		}

		//nolint:gosec // G305 checked by [inbound].
		target := filepath.Join(dstPath, header.Name)
		// Sanity check to protect against zip-slip.
		if !inbound(target, dstPath) {
			return fmt.Errorf("illegal filepath in archive: %s", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			var mode os.FileMode = 0o750

			if preserveFileMode {
				if header.Mode < 0 || header.Mode > math.MaxUint32 {
					return fmt.Errorf("invalid mode in tar header: %d", header.Mode)
				}

				mode = os.FileMode(uint32(header.Mode))
			}

			err := os.MkdirAll(target, mode)
			if err != nil {
				return fmt.Errorf("error creating nested folders: %w", err)
			}

		case tar.TypeSymlink:
			// Sanity check to protect against symlink exploit
			//nolint:gosec // G305 checked by [inbound].
			linkTarget := filepath.Join(filepath.Dir(target), header.Linkname)
			realPath, err := filepath.EvalSymlinks(linkTarget)
			if os.IsNotExist(err) {
				realPath = linkTarget
			} else if err != nil {
				return fmt.Errorf("error checking symlink realpath: %w", err)
			}

			if !inbound(realPath, dstPath) {
				return fmt.Errorf("illegal filepath in symlink: %s", linkTarget)
			}

			err = os.Symlink(realPath, target)
			if err != nil {
				return fmt.Errorf("error creating symlink: %w", err)
			}

		case tar.TypeReg:
			var mode os.FileMode = 0o644

			if preserveFileMode {
				if header.Mode < 0 || header.Mode > math.MaxUint32 {
					return fmt.Errorf("invalid mode in tar header: %d", header.Mode)
				}

				mode = os.FileMode(header.Mode)
			}

			err := os.MkdirAll(filepath.Dir(target), 0o750)
			if err != nil {
				return fmt.Errorf("error creating nested folders: %w", err)
			}

			//nolint:gosec // G304 checked by [inbound].
			f, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("error creating file %q: %w", target, err)
			}

			w := bufio.NewWriter(f)
			//nolint:gosec // G115 mitigated by [io.LimitReader].
			if _, err := io.Copy(w, tr); err != nil {
				merr := fmt.Errorf("%w: %w", ErrFailedFileWrite, err)

				if maxSize != 0 && errors.Is(err, io.ErrUnexpectedEOF) {
					merr = fmt.Errorf("%w: %w", ErrFailedFileWrite, LimitReaderUnexpectedEOFError{maxSize})
				}

				errClose := f.Close()
				if errClose != nil {
					merr = multierror.Append(merr, fmt.Errorf("%w: %w", ErrFailedFileClose, errClose))
				}

				return fmt.Errorf("error on file %q: %w", target, merr)
			}

			err = f.Close()
			if err != nil {
				return fmt.Errorf("%w %q: %w", ErrFailedFileClose, target, err)
			}
		}
	}

	return nil
}

// inbound will validate if the given candidate path is inside the
// baseDir. This is useful to make sure that malicious candidates
// are not targeting a file outside of baseDir boundaries.
// Considerations:
//   - baseDir must be absolute path. Will return false otherwise.
//   - candidate can be absolute or relative path.
//   - candidate should not be symlink as only syntactic validation is applied
//     by this function.
func inbound(candidate, baseDir string) bool {
	if !filepath.IsAbs(baseDir) {
		return false
	}

	var target string
	if filepath.IsAbs(candidate) {
		target = filepath.Clean(candidate)
	} else {
		target = filepath.Join(baseDir, candidate)
	}

	return strings.HasPrefix(target, filepath.Clean(baseDir)+string(os.PathSeparator))
}

// createTempDir will create a temporary directory in baseDir
// with CSPRNG entropy in the name to avoid clashes and mitigate
// directory traversal. If baseDir is empty string, os.TempDir()
// will be used. It is the caller's responsibility to remove the
// directory after use. Will return the full path of the generated
// directory.
func createTempDir(baseDir string) (string, error) {
	base := baseDir
	if base == "" {
		base = os.TempDir()
	}

	newUUID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("error creating directory name: %w", err)
	}

	tempDir := path.Join(base, newUUID.String())
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return "", fmt.Errorf("error creating tempDir: %w", err)
	}

	return tempDir, nil
}

func fileExists(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("error checking file existence for %q: %w", filePath, err)
	}

	return true, nil
}

func dirExists(dirPath string) bool {
	fi, err := os.Lstat(dirPath)
	if err != nil || !fi.IsDir() {
		return false
	}

	return true
}
