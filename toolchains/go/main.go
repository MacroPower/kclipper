// Reusable Go CI functions for testing, linting, and formatting.
// Provides common pipeline stages that any Go project can consume.

package main

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"dagger/go/internal/dagger"

	"github.com/bmatcuk/doublestar/v4"
)

const (
	defaultGoVersion    = "1.25" // renovate: datasource=golang-version depName=go
	golangciLintVersion = "v2.9" // renovate: datasource=github-releases depName=golangci/golangci-lint

	// defaultCacheNamespace is the default prefix for cache volume names.
	// Override via the cacheNamespace constructor parameter when consuming
	// this module from another project.
	defaultCacheNamespace = "github.com/macropower/kclipper/toolchains/go"
)

// Go provides reusable Go CI functions for testing, linting, and
// formatting. Create instances with [New].
type Go struct {
	// Go version used for base images.
	Version string
	// Project source directory.
	Source *dagger.Directory
	// Cache volume for Go module downloads (GOMODCACHE).
	ModuleCache *dagger.CacheVolume
	// Cache volume for Go build artifacts (GOCACHE).
	BuildCache *dagger.CacheVolume
	// Base container with Go installed and caches mounted. When nil in
	// the constructor, a default container is built from the official
	// golang:<version> image.
	Base *dagger.Container
	// Arguments passed to go build -ldflags.
	Ldflags []string
	// String value definitions of the form importpath.name=value,
	// added to -ldflags as -X entries.
	Values []string
	// Enable CGO.
	Cgo bool
	// Enable the race detector. Implies [Go.Cgo].
	Race bool
	// Namespace prefix for cache volume names, used to avoid collisions
	// when multiple projects consume this module.
	CacheNamespace string // +private
	// Directory containing only go.mod and go.sum, synced independently
	// of [Go.Source] so that its content hash changes only when
	// dependency files change.
	GoMod *dagger.Directory // +private
}

// New creates a [Go] module with the given project source directory.
func New(
	// Project source directory.
	// +defaultPath="/"
	source *dagger.Directory,
	// Go module files (go.mod and go.sum only). Synced separately from
	// source so that the go mod download layer is cached independently
	// of source code changes.
	// +defaultPath="/"
	// +ignore=["*", "!go.mod", "!go.sum"]
	goMod *dagger.Directory,
	// Go version for base images. Defaults to the version pinned in
	// this module.
	// +optional
	version string,
	// Cache volume for Go module downloads (GOMODCACHE). Defaults to
	// a namespaced volume named "<cacheNamespace>:modules".
	// +optional
	moduleCache *dagger.CacheVolume,
	// Cache volume for Go build artifacts (GOCACHE). Defaults to
	// a namespaced volume named "<cacheNamespace>:build".
	// +optional
	buildCache *dagger.CacheVolume,
	// Custom base container with Go installed. When provided, the
	// default golang:<version> image is not used.
	// +optional
	base *dagger.Container,
	// Arguments passed to go build -ldflags.
	// +optional
	ldflags []string,
	// String value definitions of the form importpath.name=value,
	// added to -ldflags as -X entries.
	// +optional
	values []string,
	// Enable CGO.
	// +optional
	cgo bool,
	// Enable the race detector. Implies cgo=true.
	// +optional
	race bool,
	// Namespace prefix for cache volume names. Defaults to this module's
	// canonical path. Override when consuming this module from another
	// project to avoid cache volume collisions between projects.
	// +optional
	cacheNamespace string,
) *Go {
	if version == "" {
		version = defaultGoVersion
	}
	if cacheNamespace == "" {
		cacheNamespace = defaultCacheNamespace
	}
	if moduleCache == nil {
		// Cache volumes should be namespaced by module, but they aren't (yet).
		// For now, we namespace them explicitly here.
		moduleCache = dag.CacheVolume(cacheNamespace + ":modules")
	}
	if buildCache == nil {
		// Cache volumes should be namespaced by module, but they aren't (yet).
		// For now, we namespace them explicitly here.
		buildCache = dag.CacheVolume(cacheNamespace + ":build")
	}
	if base == nil {
		base = dag.Container().
			From("golang:"+version).
			WithMountedCache("/go/pkg/mod", moduleCache).
			WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
			WithMountedCache("/go/build-cache", buildCache).
			WithEnvVariable("GOCACHE", "/go/build-cache").
			WithDirectory("/src", goMod).
			WithWorkdir("/src").
			WithExec([]string{"go", "mod", "download"})
	}
	return &Go{
		Version:        version,
		Source:         source,
		ModuleCache:    moduleCache,
		BuildCache:     buildCache,
		Base:           base,
		Ldflags:        ldflags,
		Values:         values,
		Cgo:            cgo,
		Race:           race,
		CacheNamespace: cacheNamespace,
		GoMod:          goMod,
	}
}

// ---------------------------------------------------------------------------
// Core environment
// ---------------------------------------------------------------------------

// Env returns a Go build environment container with CGO configured,
// platform env vars set, and source mounted. This is the primary entry
// point for running Go commands against the project source.
func (m *Go) Env(
	// Target platform (e.g. "linux/amd64"). When empty, uses the
	// host platform.
	// +optional
	platform dagger.Platform,
) *dagger.Container {
	src := m.Source.WithNewFile(".git/HEAD", "ref: refs/heads/main\n")

	cgoEnabled := "0"
	if m.Cgo || m.Race {
		cgoEnabled = "1"
	}

	ctr := m.Base.
		WithEnvVariable("CGO_ENABLED", cgoEnabled).
		WithMountedDirectory("/src", src)

	if platform != "" {
		parts := strings.SplitN(string(platform), "/", 3)
		if len(parts) >= 2 {
			ctr = ctr.
				WithEnvVariable("GOOS", parts[0]).
				WithEnvVariable("GOARCH", parts[1])
			if m.Cgo || m.Race {
				// Use platform-specific build cache to avoid CGO
				// cross-compilation cache pollution between architectures.
				platCache := dag.CacheVolume(m.CacheNamespace + ":build-" + parts[0] + "-" + parts[1])
				ctr = ctr.WithMountedCache("/go/build-cache", platCache)
			}
		}
	}

	return ctr
}

// Download runs go mod download using only go.mod and go.sum, warming
// the module cache for subsequent operations.
//
// +cache="session"
func (m *Go) Download(ctx context.Context) (*Go, error) {
	_, err := m.Base.Sync(ctx)
	if err != nil {
		return m, err
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Build
// ---------------------------------------------------------------------------

// Build compiles the given main packages and returns the output directory.
func (m *Go) Build(
	ctx context.Context,
	// Packages to build.
	// +optional
	// +default=["./..."]
	pkgs []string,
	// Disable symbol table.
	// +optional
	noSymbols bool,
	// Disable DWARF generation.
	// +optional
	noDwarf bool,
	// Target build platform.
	// +optional
	platform dagger.Platform,
	// Output directory path inside the container.
	// +optional
	// +default="./bin/"
	outDir string,
) (*dagger.Directory, error) {
	if m.Race {
		m.Cgo = true
	}

	ldflags := m.Ldflags
	if noSymbols {
		ldflags = append(ldflags, "-s")
	}
	if noDwarf {
		ldflags = append(ldflags, "-w")
	}

	env := m.Env(platform)
	cmd := []string{"go", "build", "-buildvcs=false", "-o", outDir}
	for _, pkg := range pkgs {
		env = env.WithExec(goCommand(cmd, []string{pkg}, ldflags, m.Values, m.Race))
	}
	return dag.Directory().WithDirectory(outDir, env.Directory(outDir)), nil
}

// Binary compiles a single main package and returns the binary file.
func (m *Go) Binary(
	ctx context.Context,
	// Package to build.
	pkg string,
	// Disable symbol table.
	// +optional
	noSymbols bool,
	// Disable DWARF generation.
	// +optional
	noDwarf bool,
	// Target build platform.
	// +optional
	platform dagger.Platform,
) (*dagger.File, error) {
	dir, err := m.Build(ctx, []string{pkg}, noSymbols, noDwarf, platform, "./bin/")
	if err != nil {
		return nil, err
	}
	files, err := dir.Glob(ctx, "bin/"+path.Base(pkg)+"*")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no matching binary for %q", pkg)
	}
	return dir.File(files[0]), nil
}

// goCommand assembles a go build/test command with ldflags, values, and
// race detector support.
func goCommand(
	cmd []string,
	pkgs []string,
	ldflags []string,
	values []string,
	race bool,
) []string {
	for _, val := range values {
		ldflags = append(ldflags, "-X '"+val+"'")
	}
	if len(ldflags) > 0 {
		cmd = append(cmd, "-ldflags", strings.Join(ldflags, " "))
	}
	if race {
		cmd = append(cmd, "-race")
	}
	cmd = append(cmd, pkgs...)
	return cmd
}

// ---------------------------------------------------------------------------
// Module scanning
// ---------------------------------------------------------------------------

// Modules returns the list of Go module directories discovered in the
// source tree. Each entry is a relative directory path (e.g. "." for the
// root module, "toolchains/go" for a nested one). Results are filtered by
// the optional include and exclude glob patterns.
func (m *Go) Modules(
	ctx context.Context,
	// Include only modules whose directory matches one of these globs.
	// An empty list matches all modules.
	// +optional
	include []string,
	// Exclude modules whose directory matches any of these globs.
	// Checked before include.
	// +optional
	exclude []string,
) ([]string, error) {
	return findModuleDirs(ctx, m.Source, include, exclude)
}

// findModuleDirs discovers Go module directories by globbing for go.mod
// files and filtering with include/exclude patterns. Dagger-related
// directories are automatically excluded: non-root directories containing
// a dagger.json (Dagger module roots) and .dagger directories (Dagger
// module runtime code). Both depend on generated SDK code that is not
// present in the source tree.
func findModuleDirs(
	ctx context.Context,
	dir *dagger.Directory,
	include, exclude []string,
) ([]string, error) {
	matches, err := dir.Glob(ctx, "**/go.mod")
	if err != nil {
		return nil, fmt.Errorf("glob go.mod: %w", err)
	}

	// Build a set of directories that contain dagger.json so we can
	// skip Dagger modules whose generated SDK code is not in source.
	daggerFiles, err := dir.Glob(ctx, "**/dagger.json")
	if err != nil {
		return nil, fmt.Errorf("glob dagger.json: %w", err)
	}
	daggerDirs := make(map[string]bool, len(daggerFiles))
	for _, df := range daggerFiles {
		daggerDirs[filepath.Dir(df)] = true
	}

	var dirs []string
	for _, match := range matches {
		modDir := filepath.Dir(match)

		// Skip Dagger-related directories: module roots (dagger.json)
		// and runtime directories (.dagger). Their generated SDK code
		// is gitignored and absent from the source directory.
		if modDir != "." && (daggerDirs[modDir] || isDaggerRuntime(modDir)) {
			continue
		}

		ok, err := filterPath(modDir, include, exclude)
		if err != nil {
			return nil, err
		}
		if ok {
			dirs = append(dirs, modDir)
		}
	}
	return dirs, nil
}

// isDaggerRuntime returns true if the path is or is inside a .dagger
// directory (Dagger module runtime code).
func isDaggerRuntime(p string) bool {
	for _, seg := range strings.Split(p, string(filepath.Separator)) {
		if seg == ".dagger" {
			return true
		}
	}
	return false
}

// filterPath returns true when path passes the include/exclude filters.
// Exclude patterns are checked first; if any match, the path is rejected.
// Then include patterns are checked; an empty include list matches all.
func filterPath(path string, include, exclude []string) (bool, error) {
	for _, pat := range exclude {
		matched, err := doublestar.PathMatch(pat, path)
		if err != nil {
			return false, fmt.Errorf("exclude pattern %q: %w", pat, err)
		}
		if matched {
			return false, nil
		}
	}
	if len(include) == 0 {
		return true, nil
	}
	for _, pat := range include {
		matched, err := doublestar.PathMatch(pat, path)
		if err != nil {
			return false, fmt.Errorf("include pattern %q: %w", pat, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// ---------------------------------------------------------------------------
// Changeset merging
// ---------------------------------------------------------------------------

// mergeChangesets combines multiple changesets into one using octopus merge.
// Nil entries are skipped.
func mergeChangesets(changesets []*dagger.Changeset) *dagger.Changeset {
	var nonNil []*dagger.Changeset
	for _, cs := range changesets {
		if cs != nil {
			nonNil = append(nonNil, cs)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	if len(nonNil) == 1 {
		return nonNil[0]
	}
	return nonNil[0].WithChangesets(nonNil[1:])
}

// ---------------------------------------------------------------------------
// Tidy
// ---------------------------------------------------------------------------

// CheckTidy verifies that go.mod and go.sum are tidy across all discovered
// Go modules by running go mod tidy per module and checking for differences.
//
// +check
func (m *Go) CheckTidy(
	ctx context.Context,
	// Include only modules whose directory matches one of these globs.
	// +optional
	include []string,
	// Exclude modules whose directory matches any of these globs.
	// +optional
	exclude []string,
) error {
	mods, err := m.Modules(ctx, include, exclude)
	if err != nil {
		return err
	}

	p := newParallel().withLimit(3)
	for _, mod := range mods {
		p = p.withJob("check-tidy:"+mod, func(ctx context.Context) error {
			changeset, err := m.TidyModule(ctx, mod)
			if err != nil {
				return err
			}
			patch, err := changeset.AsPatch().Contents(ctx)
			if err != nil {
				return err
			}
			if len(patch) > 0 {
				return fmt.Errorf("go.mod/go.sum are not tidy in %s:\n%s", mod, patch)
			}
			return nil
		})
	}
	return p.run(ctx)
}

// TidyModule runs go mod tidy for a single module directory and returns
// the changeset of go.mod/go.sum changes. The mod parameter is a relative
// directory path (e.g. "." for root, "toolchains/go" for nested).
func (m *Go) TidyModule(ctx context.Context,
	// Module directory relative to the source root.
	mod string,
) (*dagger.Changeset, error) {
	workdir := filepath.Join("/src", mod)

	tidied := m.Env("").
		WithWorkdir(workdir).
		WithExec([]string{"go", "mod", "tidy"}).
		Directory(workdir)

	modFile := filepath.Join(mod, "go.mod")
	sumFile := filepath.Join(mod, "go.sum")

	updated := m.Source.
		WithFile(modFile, tidied.File("go.mod")).
		WithFile(sumFile, tidied.File("go.sum"))

	return updated.Changes(m.Source), nil
}

// Tidy runs go mod tidy across all discovered Go modules and returns the
// merged changeset.
//
// +generate
func (m *Go) Tidy(
	ctx context.Context,
	// Include only modules whose directory matches one of these globs.
	// +optional
	include []string,
	// Exclude modules whose directory matches any of these globs.
	// +optional
	exclude []string,
) (*dagger.Changeset, error) {
	mods, err := m.Modules(ctx, include, exclude)
	if err != nil {
		return nil, err
	}

	changesets := make([]*dagger.Changeset, len(mods))
	p := newParallel().withLimit(3)
	for i, mod := range mods {
		p = p.withJob("tidy:"+mod, func(ctx context.Context) error {
			cs, err := m.TidyModule(ctx, mod)
			if err != nil {
				return err
			}
			changesets[i] = cs
			return nil
		})
	}
	if err := p.run(ctx); err != nil {
		return nil, err
	}
	return mergeChangesets(changesets), nil
}

// ---------------------------------------------------------------------------
// Base containers
// ---------------------------------------------------------------------------

// lintBase returns a golangci-lint container with source and caches. The
// Debian-based image is used (not Alpine) because it includes kernel headers
// needed by CGO transitive dependencies. The golangci-lint cache volume
// includes the linter version so that version bumps start fresh.
//
// When mod is non-empty and not ".", the container's working directory is
// set to the module subdirectory so golangci-lint operates on that module.
func (m *Go) lintBase(mod string) *dagger.Container {
	ctr := dag.Container().
		From("golangci/golangci-lint:"+golangciLintVersion).
		WithMountedCache("/go/pkg/mod", m.ModuleCache).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", m.BuildCache).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume(m.CacheNamespace+":golangci-lint-"+golangciLintVersion))

	if mod != "" && mod != "." {
		ctr = ctr.WithWorkdir(filepath.Join("/src", mod))
	}

	return ctr
}

// ---------------------------------------------------------------------------
// Helpers (public)
// ---------------------------------------------------------------------------

// EnsureGitInit ensures the container has a minimal .git directory at its
// working directory. This is sufficient for tools that only need to locate
// the repository root but do not inspect commit history or the index.
// Prefer [Go.EnsureGitRepo] when the tool requires committed files.
func (m *Go) EnsureGitInit(
	// Container to initialize.
	ctr *dagger.Container,
) *dagger.Container {
	return ctr.WithExec([]string{
		"sh", "-c",
		"if ! git rev-parse --git-dir >/dev/null 2>&1; then " +
			"rm -f .git && " +
			"git init -q; " +
			"fi",
	})
}

// EnsureGitRepo ensures the container has a valid git repository at its
// working directory with all files staged and committed. When running from
// a git worktree, the .git file references a host path that doesn't exist
// in the container. In that case, a full git repository is initialized so
// that tools like GoReleaser that depend on committed files, dirty-tree
// detection, and version derivation continue to work.
func (m *Go) EnsureGitRepo(
	// Container to initialize.
	ctr *dagger.Container,
	// Remote URL to add as origin. When empty, no remote is configured.
	// +optional
	remoteURL string,
) *dagger.Container {
	remoteCmd := ""
	if remoteURL != "" {
		remoteCmd = "git remote add origin " + remoteURL + " && "
	}
	return ctr.WithExec([]string{
		"sh", "-c",
		"if ! git rev-parse --git-dir >/dev/null 2>&1; then " +
			"rm -f .git && " +
			"git init -q && " +
			remoteCmd +
			"git add -A && " +
			"GIT_COMMITTER_DATE='2000-01-01T00:00:00+00:00' " +
			"git -c user.email=ci@dagger -c user.name=ci commit -q --allow-empty -m init " +
			"--date='2000-01-01T00:00:00+00:00'; " +
			"fi",
	})
}

