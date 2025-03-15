# kclipper/filepath

```sh
kcl mod add oci://ghcr.io/macropower/kclipper/filepath
```

The `filepath` module provides functions for manipulating file paths. These are simple wrappers for Go's `path/filepath` package. They can be helpful when used in conjunction with KCL's [`file`](https://www.kcl-lang.io/docs/reference/model/file) system package.

## Functions

| name      | type             | description                                                               |
| --------- | ---------------- | ------------------------------------------------------------------------- |
| **base**  | (str) -> str     | Returns the last element of path.                                         |
| **clean** | (str) -> str     | Returns the shortest path name equivalent to path.                        |
| **dir**   | (str) -> str     | Returns all but the last element of path, typically the path's directory. |
| **ext**   | (str) -> str     | Returns the file name extension used by path.                             |
| **join**  | (\[str\]) -> str | Joins any number of path components into a single path.                   |
| **split** | (str) -> \[str\] | Splits path into directory (\[0\]) and file name (\[1\]) components.      |

## Examples

Read `./data/example.json` relative to the file/module, regardless of the caller and/or working directory:

```py
import file
import filepath

_current_dir = filepath.dir(file.current())
_example_path = filepath.join([_current_dir, "data", "example.json"])

example_data = file.read(_example_path)
```
