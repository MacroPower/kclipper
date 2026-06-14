// Package osutil holds small filesystem helpers shared across kclipper.
package osutil

import "os"

// FileExists reports whether path names an existing regular file. Symbolic
// links are not followed, and directories return false.
func FileExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.IsDir() {
		return false
	}

	return true
}
