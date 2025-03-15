"""
This module provides an interface for the kclipper filepath plugin, which wraps
Go's `path/filepath` package.
"""
import kcl_plugin.filepath as filepath_plugin

# The following definitions are copied from Go's `path/filepath`.
# Copyright 2009 The Go Authors. All rights reserved.
# Modifications copyright 2025 Jacob Colvin.
# Licensed under the Apache License, Version 2.0.

base = lambda path: str -> str {
    """
    `base` returns the last element of path. Trailing path separators are
    removed before extracting the last element. If the path is empty, `base`
    returns `"."`. If the path consists entirely of separators, `base` returns a
    single separator.
    """
    filepath_plugin.base(path)
}

clean = lambda path: str -> str {
    """
    `clean` returns the shortest path name equivalent to path by purely lexical
    processing. It applies the following rules iteratively until no further
    processing can be done:

    1. Replace multiple slashes with a single slash.
    2. Eliminate each `.` path name element (the current directory).
    3. Eliminate each inner `..` path name element (the parent directory) along
       with the non-`..` element that precedes it.
    4. Eliminate `..` elements that begin a rooted path: that is, replace
       `"/.."` by `"/"` at the beginning of a path.

    The returned path ends in a slash only if it is the root `"/"`.

    If the result of this process is an empty string, `clean` returns the string
    `"."`.
    """
    filepath_plugin.clean(path)
}

dir = lambda path: str -> str {
    """
    `dir` returns all but the last element of path, typically the path's
    directory. After dropping the final element, `dir` calls `clean` on the path
    and trailing slashes are removed. If the path is empty, `dir` returns `"."`.
    If the path consists entirely of separators, `dir` returns a single
    separator. The returned path does not end in a separator unless it is the
    root directory.
    """
    filepath_plugin.dir(path)
}

ext = lambda path: str -> str {
    """
    `ext` returns the file name extension used by path. The extension is the
    suffix beginning at the final dot in the final element of path; it is empty
    if there is no dot.
    """
    filepath_plugin.ext(path)
}

join = lambda paths: [str] -> str {
    """
    `join` joins any number of path elements into a single path, separating them
    with slashes, and calls `clean` on the result. Empty elements are ignored.
    If the argument list is empty or all its elements are empty, `join` returns
    an empty string.
    """
    filepath_plugin.join(paths)
}

split = lambda path: str -> [str] {
    """
    `split` splits path immediately following the final slash, separating it
    into a directory and file name component. If there is no slash in path,
    `split` returns an empty dir and file set to path. The returned values have
    the property that `path = result[0] + result[1]`.
    """
    filepath_plugin.split(path)
}
