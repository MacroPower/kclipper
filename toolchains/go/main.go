// Reusable Go CI functions for testing, linting, formatting, and
// publishing. Provides common pipeline stages that any Go project can
// consume.

package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"dagger/go/internal/dagger"
)

const (
	defaultGoVersion    = "1.25"            // renovate: datasource=golang-version depName=go
	golangciLintVersion = "v2.9"            // renovate: datasource=github-releases depName=golangci/golangci-lint
	goreleaserVersion   = "v2.13.3"         // renovate: datasource=github-releases depName=goreleaser/goreleaser
	prettierVersion     = "3.5.3"           // renovate: datasource=npm depName=prettier
	zizmorVersion       = "1.22.0"          // renovate: datasource=github-releases depName=zizmorcore/zizmor
	deadcodeVersion     = "v0.42.0"         // renovate: datasource=go depName=golang.org/x/tools
	cosignVersion       = "v3.0.4"          // renovate: datasource=github-releases depName=sigstore/cosign
	syftVersion         = "v1.41.1"         // renovate: datasource=github-releases depName=anchore/syft
)

// Go provides reusable Go CI functions for testing, linting, formatting,
// and publishing. Create instances with [New].
type Go struct {
	// Go version used for base images and cache volume names.
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
	// Container image registry address (e.g. "ghcr.io/org/image").
	Registry string
	// Directory containing only go.mod and go.sum, synced independently
	// of [Go.Source] so that its content hash changes only when
	// dependency files change.
	GoMod *dagger.Directory // +private
}

// New creates a [Go] module with the given project source directory.
func New(
	// Project source directory.
	// +defaultPath="/"
	// +ignore=["dist", ".worktrees", ".tmp", ".git"]
	source *dagger.Directory,
	// Go module files (go.mod and go.sum only). Synced separately from
	// source so that the go mod download layer is cached independently
	// of source code changes.
	// +defaultPath="/"
	// +ignore=["*", "!go.mod", "!go.sum"]
	goMod *dagger.Directory,
	// Container image registry address.
	// +optional
	registry string,
	// Go version for base images and cache volume naming. Defaults to
	// the version pinned in this module.
	// +optional
	version string,
	// Cache volume for Go module downloads (GOMODCACHE). Defaults to
	// a volume named "go-mod-<version>".
	// +optional
	moduleCache *dagger.CacheVolume,
	// Cache volume for Go build artifacts (GOCACHE). Defaults to
	// a volume named "go-build-<version>".
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
) *Go {
	if version == "" {
		version = defaultGoVersion
	}
	if moduleCache == nil {
		moduleCache = dag.CacheVolume("go-mod-" + version)
	}
	if buildCache == nil {
		buildCache = dag.CacheVolume("go-build-" + version)
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
		Version:     version,
		Source:      source,
		ModuleCache: moduleCache,
		BuildCache:  buildCache,
		Base:        base,
		Ldflags:     ldflags,
		Values:      values,
		Cgo:         cgo,
		Race:        race,
		Registry:    registry,
		GoMod:       goMod,
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
// Tidy
// ---------------------------------------------------------------------------

// CheckTidy verifies that go.mod and go.sum are tidy by running
// go mod tidy and checking for differences.
//
// +check
func (m *Go) CheckTidy(ctx context.Context) error {
	changeset, err := m.tidy()
	if err != nil {
		return err
	}
	patch, err := changeset.AsPatch().Contents(ctx)
	if err != nil {
		return err
	}
	if len(patch) > 0 {
		return fmt.Errorf("go.mod/go.sum are not tidy:\n%s", patch)
	}
	return nil
}

// Tidy runs go mod tidy and returns the changeset.
//
// +generate
func (m *Go) Tidy() *dagger.Changeset {
	changeset, _ := m.tidy()
	return changeset
}

// tidy runs go mod tidy and returns the changeset of go.mod/go.sum changes.
func (m *Go) tidy() (*dagger.Changeset, error) {
	tidied := m.Env("").
		WithExec([]string{"go", "mod", "tidy"}).
		Directory("/src")

	updated := m.Source.
		WithFile("go.mod", tidied.File("go.mod")).
		WithFile("go.sum", tidied.File("go.sum"))

	return updated.Changes(m.Source), nil
}

// ---------------------------------------------------------------------------
// Base containers
// ---------------------------------------------------------------------------

// GoBase returns a Go container with source, module cache, and build cache.
// A static .git/HEAD file is injected into the source so that tools
// can locate the repository root without a container exec. Module download
// is cached via the base container.
//
// Deprecated: Use [Go.Env] instead.
func (m *Go) GoBase() *dagger.Container {
	return m.Env("")
}

// lintBase returns a golangci-lint container with source and caches. The
// Debian-based image is used (not Alpine) because it includes kernel headers
// needed by CGO transitive dependencies. The golangci-lint cache volume
// includes the linter version so that version bumps start fresh.
func (m *Go) lintBase() *dagger.Container {
	return dag.Container().
		From("golangci/golangci-lint:"+golangciLintVersion).
		WithMountedCache("/go/pkg/mod", m.ModuleCache).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", m.BuildCache).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithDirectory("/src", m.GoMod).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithMountedDirectory("/src", m.Source).
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume("golangci-lint-"+golangciLintVersion))
}

// PrettierBase returns a Node container with prettier pre-installed.
// Callers must mount their source directory and set the workdir.
func (m *Go) PrettierBase() *dagger.Container {
	return dag.Container().
		From("node:lts-slim").
		WithMountedCache("/root/.npm", dag.CacheVolume("npm-cache")).
		WithExec([]string{"npm", "install", "-g", "prettier@" + prettierVersion})
}

// GoreleaserCheckBase returns a lightweight container with only Go,
// GoReleaser, and the project source. This is sufficient for
// goreleaser check which only validates config syntax.
func (m *Go) GoreleaserCheckBase(
	// Remote URL to configure as origin. Some GoReleaser configs
	// reference a git remote; pass the repo URL when needed.
	// +optional
	remoteURL string,
) *dagger.Container {
	ctr := dag.Container().
		From("golang:"+m.Version).
		// Install GoReleaser from its official OCI image.
		WithFile("/usr/local/bin/goreleaser",
			dag.Container().From("ghcr.io/goreleaser/goreleaser:"+goreleaserVersion).
				File("/usr/bin/goreleaser")).
		WithMountedCache("/go/pkg/mod", m.ModuleCache).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", m.BuildCache).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithDirectory("/src", m.GoMod).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithMountedDirectory("/src", m.Source)
	return m.EnsureGitRepo(ctr, remoteURL)
}

// ReleaserBase returns a container with Go, GoReleaser, cosign, syft,
// module cache, build cache, and a committed git repository. This is the
// starting point for running goreleaser release with signing and SBOM
// support. Project-specific tools (cross-compilers, SDKs) can be layered
// on top by the caller.
//
// Unlike [Go.GoreleaserCheckBase], which is intentionally lightweight for
// config validation, this method installs the complete release toolset.
func (m *Go) ReleaserBase(
	// Remote URL to configure as origin.
	// +optional
	remoteURL string,
) *dagger.Container {
	ctr := dag.Container().
		From("golang:"+m.Version).
		WithFile("/usr/local/bin/goreleaser",
			dag.Container().From("ghcr.io/goreleaser/goreleaser:"+goreleaserVersion).
				File("/usr/bin/goreleaser")).
		WithFile("/usr/local/bin/cosign",
			dag.Container().From("gcr.io/projectsigstore/cosign:"+cosignVersion).
				File("/ko-app/cosign")).
		WithFile("/usr/local/bin/syft",
			dag.Container().From("ghcr.io/anchore/syft:"+syftVersion).
				File("/syft")).
		WithMountedCache("/go/pkg/mod", m.ModuleCache).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", m.BuildCache).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithDirectory("/src", m.GoMod).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithMountedDirectory("/src", m.Source)
	return m.EnsureGitRepo(ctr, remoteURL)
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

// defaultPrettierPatterns returns the default file patterns for prettier
// formatting and linting.
func defaultPrettierPatterns() []string {
	return []string{
		"*.yaml", "*.md", "*.json",
		"**/*.yaml", "**/*.md", "**/*.json",
	}
}
