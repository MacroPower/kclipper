// CI/CD functions for the kclipper project. Provides testing, linting,
// formatting, building, releasing, and development container support.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/ci/internal/dagger"

	"golang.org/x/sync/errgroup"
)

const (
	goVersion           = "1.25"            // renovate: datasource=golang-version depName=go
	golangciLintVersion = "v2.9"            // renovate: datasource=github-releases depName=golangci/golangci-lint
	goreleaserVersion   = "v2.13.3"         // renovate: datasource=github-releases depName=goreleaser/goreleaser
	zigVersion          = "0.15.2"          // renovate: datasource=github-releases depName=ziglang/zig
	cosignVersion       = "v3.0.4"          // renovate: datasource=github-releases depName=sigstore/cosign
	syftVersion         = "v1.41.1"         // renovate: datasource=github-releases depName=anchore/syft
	prettierVersion     = "3.5.3"           // renovate: datasource=npm depName=prettier
	zizmorVersion       = "1.22.0"          // renovate: datasource=github-releases depName=zizmorcore/zizmor
	kclLSPVersion       = "v0.11.2"         // renovate: datasource=github-releases depName=kcl-lang/kcl
	taskVersion         = "v3.48.0"         // renovate: datasource=github-releases depName=go-task/task
	conformVersion      = "v0.1.0-alpha.31" // renovate: datasource=github-releases depName=siderolabs/conform
	lefthookVersion     = "v2.1.1"          // renovate: datasource=github-releases depName=evilmartians/lefthook
	daggerVersion       = "v0.19.11"        // renovate: datasource=github-releases depName=dagger/dagger
	starshipVersion     = "v1.24.2"         // renovate: datasource=github-releases depName=starship/starship
	yqVersion           = "v4.52.4"         // renovate: datasource=github-releases depName=mikefarah/yq
	uvVersion           = "0.10.4"          // renovate: datasource=github-releases depName=astral-sh/uv extractVersion=^(?P<version>.*)$
	ghVersion           = "v2.87.2"         // renovate: datasource=github-releases depName=cli/cli
	claudeCodeVersion   = "1.0.58"          // renovate: datasource=npm depName=@anthropic-ai/claude-code

	defaultRegistry = "ghcr.io/macropower/kclipper"

	// macosSDKFlags are the common compiler flags for macOS cross-compilation
	// via Zig, pointing to the vendored macOS SDK headers.
	macosSDKFlags = "-F/sdk/MacOSX.sdk/System/Library/Frameworks " +
		"-I/sdk/MacOSX.sdk/usr/include " +
		"-L/sdk/MacOSX.sdk/usr/lib " +
		"-Wno-availability -Wno-nullability-completeness"
)

// Ci provides CI/CD functions for kclipper. Create instances with [New].
type Ci struct {
	// Project source directory.
	Source *dagger.Directory
	// Directory containing only go.mod and go.sum, synced independently of
	// [Ci.Source] so that its content hash changes only when dependency
	// files change. Used by [Ci.goModBase] to cache go mod download.
	GoMod *dagger.Directory
	// Container image registry address (e.g. "ghcr.io/macropower/kclipper").
	Registry string
}

// New creates a [Ci] module with the given project source directory.
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
) *Ci {
	if registry == "" {
		registry = defaultRegistry
	}
	return &Ci{Source: source, GoMod: goMod, Registry: registry}
}

// ---------------------------------------------------------------------------
// Testing
// ---------------------------------------------------------------------------

// Test runs Go tests as a fast pre-commit check. Race detection and vet
// analysis are omitted here because [Ci.TestCoverage] (used in CI) includes
// both, and [Ci.Lint] already runs govet via golangci-lint.
//
// +check
func (m *Ci) Test(ctx context.Context) error {
	_, err := m.goBase().
		WithExec([]string{
			"go", "test", "-race", "./...",
		}).
		Sync(ctx)
	return err
}

// TestCoverage runs Go tests and returns the coverage profile file.
func (m *Ci) TestCoverage() *dagger.File {
	return m.goBase().
		WithExec([]string{
			"go", "test", "-race", "-vet=all",
			"-coverprofile=/tmp/coverage.txt", "./...",
		}).
		File("/tmp/coverage.txt")
}

// ---------------------------------------------------------------------------
// Linting
// ---------------------------------------------------------------------------

// Lint runs golangci-lint on the source code.
//
// +check
func (m *Ci) Lint(ctx context.Context) error {
	_, err := m.lintBase().
		WithExec([]string{"golangci-lint", "run"}).
		Sync(ctx)
	return err
}

// LintPrettier checks YAML, JSON, and Markdown formatting.
//
// +check
func (m *Ci) LintPrettier(ctx context.Context) error {
	_, err := prettierBase().
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec([]string{
			"prettier", "--config", "./.prettierrc.yaml", "--check",
			"*.yaml", "*.md", "*.json",
			"**/*.yaml", "**/*.md", "**/*.json",
		}).
		Sync(ctx)
	return err
}

// LintActions runs zizmor to lint GitHub Actions workflows.
//
// +check
func (m *Ci) LintActions(ctx context.Context) error {
	_, err := dag.Container().
		From("ghcr.io/zizmorcore/zizmor:"+zizmorVersion).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithExec([]string{
			"zizmor", ".github/workflows", "--config", ".github/zizmor.yaml",
		}).
		Sync(ctx)
	return err
}

// LintReleaser validates the GoReleaser configuration.
//
// +check
func (m *Ci) LintReleaser(ctx context.Context) error {
	_, err := m.releaserBase().
		WithExec([]string{"goreleaser", "check"}).
		Sync(ctx)
	return err
}

// LintCommitMsg validates a commit message against the project's conventional
// commit policy using conform. The message file is typically provided by a
// git commit-msg hook.
func (m *Ci) LintCommitMsg(
	ctx context.Context,
	// Commit message file to validate (e.g. .git/COMMIT_EDITMSG).
	msgFile *dagger.File,
) error {
	ctr := dag.Container().
		From("alpine/git:latest").
		WithFile("/usr/local/bin/conform",
			dag.Container().From("ghcr.io/siderolabs/conform:"+conformVersion).
				File("/conform")).
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src")

	_, err := ensureGitInit(ctr).
		WithMountedFile("/tmp/commit-msg", msgFile).
		WithExec([]string{"conform", "enforce", "--commit-msg-file", "/tmp/commit-msg"}).
		Sync(ctx)
	return err
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

// Format runs golangci-lint --fix and prettier --write, returning the
// changeset against the original source directory.
//
// +generate
func (m *Ci) Format() *dagger.Changeset {
	// Go formatting via golangci-lint --fix.
	goFmt := m.lintBase().
		WithExec([]string{"golangci-lint", "run", "--fix"}).
		Directory("/src")

	// Prettier formatting.
	formatted := prettierBase().
		WithMountedDirectory("/src", goFmt).
		WithWorkdir("/src").
		WithExec([]string{
			"prettier", "--config", "./.prettierrc.yaml", "-w",
			"*.yaml", "*.md", "*.json",
			"**/*.yaml", "**/*.md", "**/*.json",
		}).
		Directory("/src")

	return formatted.Changes(m.Source)
}

// ---------------------------------------------------------------------------
// Building
// ---------------------------------------------------------------------------

// Build runs GoReleaser in snapshot mode, producing binaries for all
// platforms. Returns the dist/ directory.
func (m *Ci) Build() *dagger.Directory {
	return m.releaserBase().
		WithExec([]string{
			"goreleaser", "release", "--snapshot", "--clean",
			"--skip=docker,homebrew,nix,sign,sbom",
		}).
		Directory("/src/dist")
}

// VersionTags returns the image tags derived from a version tag string.
// For example, "v1.2.3" yields ["latest", "v1.2.3", "v1", "v1.2"].
func (m *Ci) VersionTags(
	// Version tag (e.g. "v1.2.3").
	tag string,
) []string {
	v := strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(v, ".", 3)

	// Detect pre-release: any version component contains a hyphen
	// (e.g. "1.0.0-rc.1" → third part is "0-rc.1").
	for _, p := range parts {
		if strings.Contains(p, "-") {
			return []string{tag}
		}
	}

	tags := []string{"latest", tag}
	if len(parts) >= 1 {
		tags = append(tags, "v"+parts[0])
	}
	if len(parts) >= 2 {
		tags = append(tags, "v"+parts[0]+"."+parts[1])
	}
	return tags
}

// FormatDigestChecksums converts [Ci.PublishImages] output references to the
// checksums format expected by actions/attest-build-provenance. Each reference
// has the form "registry/image:tag@sha256:hex"; this function emits
// "hex  registry/image:tag" lines, deduplicating by digest.
func (m *Ci) FormatDigestChecksums(
	// Image references from [Ci.PublishImages] (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) string {
	seen := make(map[string]bool)
	var b strings.Builder
	for _, ref := range refs {
		parts := strings.SplitN(ref, "@sha256:", 2)
		if len(parts) != 2 {
			continue
		}
		hex := parts[1]
		if seen[hex] {
			continue
		}
		seen[hex] = true
		fmt.Fprintf(&b, "%s  %s\n", hex, parts[0])
	}
	return b.String()
}

// DeduplicateDigests returns unique image references from a list, keeping
// only the first occurrence of each sha256 digest.
func (m *Ci) DeduplicateDigests(
	// Image references (e.g. "registry/image:tag@sha256:hex").
	refs []string,
) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, ref := range refs {
		parts := strings.SplitN(ref, "@sha256:", 2)
		if len(parts) != 2 {
			continue
		}
		if !seen[parts[1]] {
			seen[parts[1]] = true
			unique = append(unique, ref)
		}
	}
	return unique
}

// RegistryHost extracts the host (with optional port) from a registry
// address. For example, "ghcr.io/macropower/kclipper" returns "ghcr.io".
func (m *Ci) RegistryHost(
	// Registry address (e.g. "ghcr.io/macropower/kclipper").
	registry string,
) string {
	return strings.SplitN(registry, "/", 2)[0]
}

// BuildImages builds multi-arch runtime container images from a GoReleaser
// dist directory. If no dist is provided, a snapshot build is run.
func (m *Ci) BuildImages(
	// Version label for OCI metadata.
	// +default="snapshot"
	version string,
	// Pre-built GoReleaser dist directory. If not provided, runs a snapshot build.
	// +optional
	dist *dagger.Directory,
) []*dagger.Container {
	if dist == nil {
		dist = m.Build()
	}
	return runtimeImages(dist, version)
}

// PublishImages builds multi-arch container images using Dagger's native
// Container API and publishes them to the registry.
//
// Stable releases are published with multiple tags: :latest, :vX.Y.Z, :vX,
// :vX.Y. Pre-release versions are published with only their exact tag.
//
// +cache="never"
func (m *Ci) PublishImages(
	ctx context.Context,
	// Image tags to publish (e.g. ["latest", "v1.2.3", "v1", "v1.2"]).
	tags []string,
	// Registry username for authentication.
	// +optional
	registryUsername string,
	// Registry password or token for authentication.
	// +optional
	registryPassword *dagger.Secret,
	// Cosign private key for signing published images.
	// +optional
	cosignKey *dagger.Secret,
	// Password for the cosign private key. Required when the key is encrypted.
	// +optional
	cosignPassword *dagger.Secret,
	// Pre-built GoReleaser dist directory. If not provided, runs a snapshot build.
	// +optional
	dist *dagger.Directory,
) (string, error) {
	// Use the first non-"latest" tag as the version label, or fall back to "snapshot".
	version := "snapshot"
	for _, t := range tags {
		if t != "latest" {
			version = t
			break
		}
	}

	variants := m.BuildImages(version, dist)
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword, cosignKey, cosignPassword)
	if err != nil {
		return "", err
	}

	// Deduplicate digests for the summary (tags may share a manifest).
	unique := m.DeduplicateDigests(digests)
	return fmt.Sprintf("published %d tags (%d unique digests)\n%s", len(tags), len(unique), strings.Join(digests, "\n")), nil
}

// publishImages publishes pre-built container image variants to the registry
// and returns the list of published image digests.
//
// This is the internal implementation shared by [Ci.PublishImages] and
// [Ci.Release].
func (m *Ci) publishImages(
	ctx context.Context,
	variants []*dagger.Container,
	tags []string,
	registryUsername string,
	registryPassword *dagger.Secret,
	cosignKey *dagger.Secret,
	cosignPassword *dagger.Secret,
) ([]string, error) {
	// Publish multi-arch manifest for each tag concurrently.
	publisher := dag.Container()
	if registryPassword != nil {
		publisher = publisher.WithRegistryAuth(m.RegistryHost(m.Registry), registryUsername, registryPassword)
	}

	digests := make([]string, len(tags))
	g, gCtx := errgroup.WithContext(ctx)
	for i, t := range tags {
		ref := fmt.Sprintf("%s:%s", m.Registry, t)
		g.Go(func() error {
			digest, err := publisher.Publish(gCtx, ref, dagger.ContainerPublishOpts{
				PlatformVariants: variants,
			})
			if err != nil {
				return fmt.Errorf("publish %s: %w", ref, err)
			}
			digests[i] = digest
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Sign each published image with cosign (key-based signing).
	// Deduplicate first -- multiple tags often share one manifest digest.
	if cosignKey != nil {
		toSign := m.DeduplicateDigests(digests)

		cosignCtr := dag.Container().
			From("gcr.io/projectsigstore/cosign:"+cosignVersion).
			WithSecretVariable("COSIGN_KEY", cosignKey)
		if registryPassword != nil {
			cosignCtr = cosignCtr.WithRegistryAuth(m.RegistryHost(m.Registry), registryUsername, registryPassword)
		}
		if cosignPassword != nil {
			cosignCtr = cosignCtr.WithSecretVariable("COSIGN_PASSWORD", cosignPassword)
		}

		g, gCtx := errgroup.WithContext(ctx)
		for _, digest := range toSign {
			g.Go(func() error {
				_, err := cosignCtr.
					WithExec([]string{"cosign", "sign", "--key", "env://COSIGN_KEY", digest, "--yes"}).
					Sync(gCtx)
				if err != nil {
					return fmt.Errorf("sign image %s: %w", digest, err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	return digests, nil
}

// Release runs GoReleaser for binaries/archives/signing, then builds and
// publishes container images using Dagger-native Container.Publish().
// GoReleaser's Docker support is skipped entirely to avoid Docker-in-Docker.
//
// Returns the dist/ directory containing checksums.txt and digests.txt
// for attestation in the calling workflow.
//
// +cache="never"
func (m *Ci) Release(
	ctx context.Context,
	// GitHub token for creating the release.
	githubToken *dagger.Secret,
	// Registry username for container image authentication.
	registryUsername string,
	// Registry password or token for container image authentication.
	registryPassword *dagger.Secret,
	// Version tag to release (e.g. "v1.2.3").
	tag string,
	// Cosign private key for signing published images.
	// +optional
	cosignKey *dagger.Secret,
	// Password for the cosign private key. Required when the key is encrypted.
	// +optional
	cosignPassword *dagger.Secret,
	// macOS code signing PKCS#12 certificate (base64-encoded).
	// +optional
	macosSignP12 *dagger.Secret,
	// Password for the macOS code signing certificate.
	// +optional
	macosSignPassword *dagger.Secret,
	// Apple App Store Connect API key for notarization.
	// +optional
	macosNotaryKey *dagger.Secret,
	// Apple App Store Connect API key ID.
	// +optional
	macosNotaryKeyId *dagger.Secret,
	// Apple App Store Connect API issuer ID.
	// +optional
	macosNotaryIssuerId *dagger.Secret,
) (*dagger.Directory, error) {
	ctr := m.releaserBase().
		WithSecretVariable("GITHUB_TOKEN", githubToken)

	// Conditionally add cosign secrets for GoReleaser binary signing.
	// When no key is provided, signing is skipped entirely since keyless
	// OIDC signing is unavailable inside the Dagger container.
	skipFlags := "docker"
	if cosignKey != nil {
		ctr = ctr.WithSecretVariable("COSIGN_KEY", cosignKey)
		if cosignPassword != nil {
			ctr = ctr.WithSecretVariable("COSIGN_PASSWORD", cosignPassword)
		}
	} else {
		skipFlags = "docker,sign"
	}

	// Conditionally add macOS signing secrets.
	if macosSignP12 != nil {
		ctr = ctr.
			WithSecretVariable("MACOS_SIGN_P12", macosSignP12).
			WithSecretVariable("MACOS_SIGN_PASSWORD", macosSignPassword).
			WithSecretVariable("MACOS_NOTARY_KEY", macosNotaryKey).
			WithSecretVariable("MACOS_NOTARY_KEY_ID", macosNotaryKeyId).
			WithSecretVariable("MACOS_NOTARY_ISSUER_ID", macosNotaryIssuerId)
	}

	// Run GoReleaser for binaries, archives, Homebrew, Nix (and signing
	// when cosignKey is provided). Docker is always skipped -- images are
	// published natively via Dagger below.
	dist := ctr.
		WithExec([]string{"goreleaser", "release", "--clean", "--skip=" + skipFlags}).
		Directory("/src/dist")

	// Derive image tags from the version tag (e.g. v1.2.3 -> latest, v1.2.3, v1, v1.2).
	tags := m.VersionTags(tag)

	// Publish multi-arch container images via Dagger-native API.
	// Reuse the dist directory from the goreleaser run to avoid building twice.
	variants := runtimeImages(dist, tag)
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword, cosignKey, cosignPassword)
	if err != nil {
		return nil, fmt.Errorf("publish images: %w", err)
	}

	// Write digests in checksums format for attest-build-provenance.
	// Dagger's Publish returns "registry/image:tag@sha256:hex" but the
	// action's subject-checksums input expects "hex  name" per sha256sum.
	if len(digests) > 0 {
		dist = dist.WithNewFile("digests.txt", m.FormatDigestChecksums(digests))
	}

	return dist, nil
}

// ---------------------------------------------------------------------------
// Development
// ---------------------------------------------------------------------------

// starshipConfig is the starship prompt configuration written to
// ~/.config/starship.toml inside the dev container.
const starshipConfig = `add_newline = false
palette = 'one_dark'
format = "$directory$git_branch$git_status$golang$fill$cmd_duration$line_break$character"

[fill]
symbol = ' '

[directory]
truncation_length = 3
style = 'bold blue'

[git_branch]
format = '[$symbol$branch]($style) '
symbol = '@ '
style = 'bold purple'

[git_status]
format = '([$all_status$ahead_behind]($style) )'
style = 'bold yellow'

[golang]
format = '[$symbol$version]($style) '
symbol = 'go '
style = 'bold cyan'

[cmd_duration]
min_time = 2_000
format = '[$duration]($style)'
style = 'comment'

[character]
success_symbol = '[>](bold green)'
error_symbol = '[>](bold red)'

[palettes.one_dark]
red = '#E06C75'
green = '#98C379'
yellow = '#E5C07B'
blue = '#61AFEF'
purple = '#C678DD'
cyan = '#56B6C2'
white = '#ABB2BF'
comment = '#5C6370'
`

// zshConfig is the zsh configuration written to ~/.zshrc inside the dev
// container.
const zshConfig = `# History (persisted via cache volume)
HISTFILE=/commandhistory/.zsh_history
HISTSIZE=10000
SAVEHIST=10000
setopt HIST_IGNORE_ALL_DUPS SHARE_HISTORY APPEND_HISTORY INC_APPEND_HISTORY

# Completions
autoload -Uz compinit && compinit
zstyle ':completion:*' menu select
zstyle ':completion:*' matcher-list 'm:{a-z}={A-Z}'
zstyle ':completion:*' list-colors "${(s.:.)LS_COLORS}"

# Plugins
source /usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE='fg=244'
source /usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh

# Colors
eval "$(dircolors -b)"

# fzf integration
source /usr/share/doc/fzf/examples/key-bindings.zsh
source /usr/share/doc/fzf/examples/completion.zsh
export FZF_DEFAULT_COMMAND='fd --type f --hidden --follow --exclude .git'
export FZF_DEFAULT_OPTS='--height=40% --layout=reverse --border --color=fg:-1,bg:-1,hl:cyan,fg+:white,bg+:236,hl+:cyan,info:yellow,prompt:green,pointer:magenta,marker:magenta'

# Tool config
export BAT_THEME='ansi'

# Aliases
alias ls='ls --color=auto'
alias ll='ls -lh'
alias la='ls -lAh'
alias l='ls -CF'
alias grep='grep --color=auto'
alias cat='bat --paging=never'

# direnv
eval "$(direnv hook zsh)"

# Starship prompt (must be last)
eval "$(starship init zsh)"
`

// devInitScript is the shell script that initializes the git repository
// and overlays local source files in the dev container. It expects BRANCH
// and BASE environment variables to be set.
const devInitScript = "" +
	// Clone if needed (blobless: full history, blobs fetched on demand).
	"if [ ! -d /src/.git ]; then " +
	"git clone --filter=blob:none --no-checkout " +
	"https://github.com/macropower/kclipper.git /src; " +
	"fi && " +
	"cd /src && git fetch origin && " +
	// Checkout or create branch.
	"if git rev-parse --verify \"${BRANCH}\" >/dev/null 2>&1; then " +
	"git checkout \"${BRANCH}\"; " +
	"elif git rev-parse --verify \"origin/${BRANCH}\" >/dev/null 2>&1; then " +
	"git checkout -b \"${BRANCH}\" \"origin/${BRANCH}\"; " +
	"else " +
	"git checkout -b \"${BRANCH}\" \"origin/${BASE}\"; " +
	"fi && " +
	// Overlay local source (m.Source excludes .git via +ignore).
	// rsync --delete removes files present in git but deleted locally.
	"rsync -a --delete --exclude=.git /tmp/src-seed/ /src/"

// DevEnv returns a development container with the git repository cloned,
// the requested branch checked out, and local source files overlaid.
// Cache volumes provide per-branch workspace isolation and shared Go
// module/build caches. Unlike [Ci.Dev], this does not open an interactive
// terminal or export results.
func (m *Ci) DevEnv(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
) *dagger.Container {
	if base == "" {
		base = "main"
	}

	return devBase().
		// Stage source on regular filesystem for the seed step.
		WithDirectory("/tmp/src-seed", m.Source).
		// Cache volume at /src so changes survive Terminal().
		// Each branch gets its own volume for workspace isolation.
		WithMountedCache("/src", dag.CacheVolume("dev-src-"+sanitizeCacheKey(branch))).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithMountedCache("/commandhistory", dag.CacheVolume("shell-history")).
		WithWorkdir("/src").
		WithEnvVariable("BRANCH", branch).
		WithEnvVariable("BASE", base).
		WithEnvVariable("_DEV_TS", time.Now().String()).
		WithExec([]string{"sh", "-c", devInitScript})
}

// Dev opens an interactive development container with a real git
// repository and returns the modified source directory when the session
// ends. The container is created via [Ci.DevEnv], which clones the
// upstream repo (blobless) and checks out the specified branch, enabling
// pushes, rebases, and other git operations.
//
// Source files from the project directory are overlaid on top of the
// checked-out branch, bringing in local uncommitted changes. Each branch
// gets its own Dagger cache volume for workspace isolation.
//
// The returned directory includes .git with full commit history. Use the
// Taskfile dev/claude tasks to handle the translation between the
// container's standalone .git and the host's worktree format.
//
// Usage:
//
//	task dev                        # defaults to current branch
//	task dev BRANCH=feat/my-work    # explicit branch, base = current branch
//	task claude BRANCH=feat/my-work BASE=main  # explicit base
//
// +cache="never"
func (m *Ci) Dev(
	// Branch to check out in the dev container. Each branch gets its
	// own Dagger cache volume for workspace isolation.
	branch string,
	// Base branch name used when creating a new branch that does not
	// exist locally or on the remote. Looked up as origin/<base> in
	// the container clone. Defaults to "main" when empty.
	// +optional
	base string,
	// Claude Code configuration directory (~/.claude).
	// +optional
	claudeConfig *dagger.Directory,
	// Claude Code settings file (~/.claude.json).
	// +optional
	claudeJSON *dagger.File,
	// Git configuration directory (~/.config/git).
	// +optional
	gitConfig *dagger.Directory,
	// Claude Code status line configuration directory (~/.config/ccstatusline).
	// +optional
	ccstatuslineConfig *dagger.Directory,
	// Timezone for the container (e.g. "America/New_York").
	// +optional
	tz string,
	// COLORTERM value (e.g. "truecolor").
	// +optional
	colorterm string,
	// TERM_PROGRAM value (e.g. "Apple_Terminal", "iTerm.app").
	// +optional
	termProgram string,
	// TERM_PROGRAM_VERSION value.
	// +optional
	termProgramVersion string,
	// Command to run in the terminal session. Defaults to ["zsh"].
	// +optional
	cmd []string,
) *dagger.Directory {
	ctr := m.DevEnv(branch, base)

	ctr = applyDevConfig(ctr, claudeConfig, claudeJSON, gitConfig, ccstatuslineConfig,
		tz, colorterm, termProgram, termProgramVersion)

	// Pre-download Go modules (non-fatal: user can fix go.mod interactively).
	ctr = ctr.WithExec([]string{"sh", "-c",
		"go mod download || echo 'WARNING: go mod download failed; run it manually' >&2",
	})

	// Open interactive terminal. Changes to /src persist in the cache
	// volume through the Terminal() call.
	if len(cmd) == 0 {
		cmd = []string{"zsh"}
	}
	ctr = ctr.Terminal(dagger.ContainerTerminalOpts{
		Cmd:                           cmd,
		ExperimentalPrivilegedNesting: true,
	})

	// Copy from cache volume to regular filesystem so Directory() can
	// read it (Container.Directory rejects cache mount paths).
	ctr = ctr.WithExec([]string{"sh", "-c", "mkdir -p /output && cp -a /src/. /output/"})

	return ctr.Directory("/output")
}

// applyDevConfig applies optional configuration mounts and environment
// variables to a dev container.
func applyDevConfig(
	ctr *dagger.Container,
	claudeConfig *dagger.Directory,
	claudeJSON *dagger.File,
	gitConfig *dagger.Directory,
	ccstatuslineConfig *dagger.Directory,
	tz, colorterm, termProgram, termProgramVersion string,
) *dagger.Container {
	if claudeConfig != nil {
		ctr = ctr.WithMountedDirectory("/root/.claude", claudeConfig)
	}
	if claudeJSON != nil {
		ctr = ctr.WithMountedFile("/root/.claude.json", claudeJSON)
	}
	if gitConfig != nil {
		ctr = ctr.WithMountedDirectory("/root/.config/git", gitConfig)
	}
	if ccstatuslineConfig != nil {
		ctr = ctr.WithMountedDirectory("/root/.config/ccstatusline", ccstatuslineConfig)
	}
	if tz != "" {
		ctr = ctr.WithEnvVariable("TZ", tz)
	}
	if colorterm != "" {
		ctr = ctr.WithEnvVariable("COLORTERM", colorterm)
	}
	if termProgram != "" {
		ctr = ctr.WithEnvVariable("TERM_PROGRAM", termProgram)
	}
	if termProgramVersion != "" {
		ctr = ctr.WithEnvVariable("TERM_PROGRAM_VERSION", termProgramVersion)
	}
	return ctr
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// runtimeImages builds a multi-arch set of runtime container images from a
// pre-built GoReleaser dist/ directory. Each image is based on debian:13-slim
// with OCI labels, KCL environment variables, and runtime dependencies.
func runtimeImages(dist *dagger.Directory, version string) []*dagger.Container {
	platforms := []dagger.Platform{"linux/amd64", "linux/arm64"}
	variants := make([]*dagger.Container, 0, len(platforms))
	created := time.Now().UTC().Format(time.RFC3339)

	for _, platform := range platforms {
		// Map platform to GoReleaser dist binary path.
		// GoReleaser uses the build id (kclipper), not the binary name (kcl).
		// Directory names include the GOAMD64/GOARM64 version suffix:
		//   amd64 -> kclipper_linux_amd64_v1
		//   arm64 -> kclipper_linux_arm64_v8.0
		dir := "kclipper_linux_amd64_v1"
		if platform == "linux/arm64" {
			dir = "kclipper_linux_arm64_v8.0"
		}

		ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
			From("debian:13-slim").
			// OCI labels (container config) for metadata.
			WithLabel("org.opencontainers.image.title", "kclipper").
			WithLabel("org.opencontainers.image.description", "A superset of KCL that integrates Helm chart management").
			WithLabel("org.opencontainers.image.source", "https://github.com/macropower/kclipper").
			WithLabel("org.opencontainers.image.url", "https://github.com/macropower/kclipper").
			WithLabel("org.opencontainers.image.version", version).
			WithLabel("org.opencontainers.image.created", created).
			WithLabel("org.opencontainers.image.licenses", "Apache-2.0").
			// OCI annotations (manifest-level) for registry discoverability.
			WithAnnotation("org.opencontainers.image.title", "kclipper").
			WithAnnotation("org.opencontainers.image.version", version).
			WithAnnotation("org.opencontainers.image.created", created).
			WithAnnotation("org.opencontainers.image.source", "https://github.com/macropower/kclipper").
			// Match env vars from existing Dockerfile.
			WithEnvVariable("LANG", "en_US.utf8").
			WithEnvVariable("XDG_CACHE_HOME", "/tmp/xdg_cache").
			WithEnvVariable("KCL_LIB_HOME", "/tmp/kcl_lib").
			WithEnvVariable("KCL_PKG_PATH", "/tmp/kcl_pkg").
			WithEnvVariable("KCL_CACHE_PATH", "/tmp/kcl_cache").
			WithEnvVariable("KCL_FAST_EVAL", "1").
			// Install runtime dependencies (curl/gpg for plugin installs).
			WithExec([]string{"sh", "-c",
				"apt-get update && apt-get install -y curl gpg apt-transport-https && rm -rf /var/lib/apt/lists/* /tmp/*"}).
			WithFile("/usr/local/bin/kcl", dist.File(dir+"/kcl")).
			WithEntrypoint([]string{"kcl"})

		variants = append(variants, ctr)
	}

	return variants
}

// DevBase returns the base development container with all tools
// pre-installed but no source mounted. Used by integration tests to
// verify tool availability without requiring an interactive terminal.
func (m *Ci) DevBase() *dagger.Container {
	return devBase()
}

// ---------------------------------------------------------------------------
// Dev container base & tool builders (private helpers)
// ---------------------------------------------------------------------------

// devBase returns the base development container with all tools
// pre-installed. All tool binaries are consolidated into a single
// builder container ([devToolBins]) so Dagger resolves one
// sub-pipeline instead of one per tool. Claude Code is a separate
// directory install ([claudeCodeFiles]).
func devBase() *dagger.Container {
	return dag.Container().
		From("golang:"+goVersion).
		// Mount apt cache volumes so re-runs skip network downloads.
		WithMountedCache("/var/cache/apt/archives", dag.CacheVolume("dev-apt-archives")).
		WithMountedCache("/var/lib/apt/lists", dag.CacheVolume("dev-apt-lists")).
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y --no-install-recommends " +
				"curl less man-db gnupg2 nano vim xz-utils jq wget dnsutils direnv " +
				"zsh zsh-autosuggestions zsh-syntax-highlighting " +
				"ripgrep fd-find bat fzf tree htop rsync",
		}).
		// Symlink Debian-renamed binaries to their canonical names.
		WithExec([]string{"sh", "-c",
			"ln -s /usr/bin/batcat /usr/local/bin/bat && " +
				"ln -s /usr/bin/fdfind /usr/local/bin/fd",
		}).
		// All tool binaries from a single builder sub-pipeline.
		WithDirectory("/usr/local/bin", devToolBins()).
		WithDirectory("/root/.local", claudeCodeFiles()).
		// Shell config.
		WithNewFile("/root/.config/starship.toml", starshipConfig).
		WithNewFile("/root/.zshrc", zshConfig).
		// Editor and terminal env vars.
		WithEnvVariable("EDITOR", "nano").
		WithEnvVariable("VISUAL", "nano").
		WithEnvVariable("TERM", "xterm-256color").
		WithEnvVariable("PATH", "/root/.local/bin:$PATH",
			dagger.ContainerWithEnvVariableOpts{Expand: true})
}

// devToolBins returns a directory containing all dev tool binaries.
// Everything is built in a single alpine container so Dagger resolves
// one sub-pipeline for all tools. GitHub release downloads run in one
// exec; OCI image binaries are added via [dagger.Container.WithFile].
func devToolBins() *dagger.Directory {
	ghVer := strings.TrimPrefix(ghVersion, "v")
	lefthookVer := strings.TrimPrefix(lefthookVersion, "v")
	return dag.Container().
		From("alpine:3").
		WithExec([]string{"mkdir", "-p", "/tools"}).
		// Download all GitHub release tools in one exec.
		WithExec([]string{"sh", "-c",
			"ARCH=$(uname -m) && " +
				"GOARCH=$(echo $ARCH | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && " +
				// starship
				"wget -qO- https://github.com/starship/starship/releases/download/" + starshipVersion +
				"/starship-${ARCH}-unknown-linux-musl.tar.gz | tar xz -C /tools && " +
				// task
				"wget -qO- https://github.com/go-task/task/releases/download/" + taskVersion +
				"/task_linux_${GOARCH}.tar.gz | tar xz -C /tools task && " +
				// lefthook
				"wget -qO /tools/lefthook https://github.com/evilmartians/lefthook/releases/download/" + lefthookVersion +
				"/lefthook_" + lefthookVer + "_Linux_${ARCH} && chmod +x /tools/lefthook && " +
				// gh
				"wget -qO- https://github.com/cli/cli/releases/download/" + ghVersion +
				"/gh_" + ghVer + "_linux_${GOARCH}.tar.gz | " +
				"tar xz -O gh_" + ghVer + "_linux_${GOARCH}/bin/gh > /tools/gh && chmod +x /tools/gh",
		}).
		// OCI image binaries.
		WithFile("/tools/conform",
			dag.Container().From("ghcr.io/siderolabs/conform:"+conformVersion).
				File("/conform")).
		WithFile("/tools/dagger",
			dag.Container().From("registry.dagger.io/engine:"+daggerVersion).
				File("/usr/local/bin/dagger")).
		WithFile("/tools/yq",
			dag.Container().From("mikefarah/yq:"+strings.TrimPrefix(yqVersion, "v")).
				File("/usr/bin/yq")).
		WithFile("/tools/uv",
			dag.Container().From("ghcr.io/astral-sh/uv:"+uvVersion).
				File("/uv")).
		Directory("/tools")
}

// claudeCodeFiles returns the Claude Code installation directory from a
// pinned install script run inside a debian-slim builder.
func claudeCodeFiles() *dagger.Directory {
	return dag.Container().
		From("debian:13-slim").
		WithExec([]string{"sh", "-c",
			"apt-get update && apt-get install -y --no-install-recommends curl ca-certificates",
		}).
		WithExec([]string{"sh", "-c",
			"curl -fsSL https://claude.ai/install.sh | bash -s -- " + claudeCodeVersion,
		}).
		Directory("/root/.local")
}

// ---------------------------------------------------------------------------
// Base containers (private helpers)
// ---------------------------------------------------------------------------

// prettierBase returns a Node container with prettier pre-installed.
// Callers must mount their source directory and set the workdir.
func prettierBase() *dagger.Container {
	return dag.Container().
		From("node:lts-slim").
		WithMountedCache("/root/.npm", dag.CacheVolume("npm-cache")).
		WithExec([]string{"npm", "install", "-g", "prettier@" + prettierVersion})
}

// goModBase mounts Go module and build cache volumes, copies [Ci.GoMod]
// (go.mod and go.sum only) into the container, and runs go mod download.
// Because [Ci.GoMod] is synced independently of [Ci.Source], its content
// hash changes only when dependency files change, not on every source edit.
// The cache volumes are mounted before the download so that go mod download
// is a near-instant no-op when modules are already present in the persistent
// volume. The full source directory is mounted last. Both [Ci.lintBase] and
// [Ci.goBase] delegate to this method.
func (m *Ci) goModBase(ctr *dagger.Container, src *dagger.Directory) *dagger.Container {
	return ctr.
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithDirectory("/src", m.GoMod).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"}).
		WithMountedDirectory("/src", src)
}

// lintBase returns a golangci-lint container with source and caches. The
// Debian-based image is used (not Alpine) because it includes kernel headers
// needed by CGO transitive dependencies (e.g. containers/storage). Module
// download is cached via [Ci.goModBase].
func (m *Ci) lintBase() *dagger.Container {
	ctr := dag.Container().
		From("golangci/golangci-lint:" + golangciLintVersion)
	return m.goModBase(ctr, m.Source).
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume("golangci-lint"))
}

// goBase returns a Go container with source, module cache, and build cache.
// A static .git/HEAD file is injected into the source so that [FindRepoRoot]
// can locate the repository root without a container exec. Module download is
// cached via [Ci.goModBase].
func (m *Ci) goBase() *dagger.Container {
	src := m.Source.WithNewFile(".git/HEAD", "ref: refs/heads/main\n")
	ctr := dag.Container().
		From("golang:"+goVersion).
		WithEnvVariable("CGO_ENABLED", "1")
	return m.goModBase(ctr, src)
}

// releaserBase returns a container with Go, GoReleaser, Zig, cosign, syft,
// pre-downloaded KCL LSP binaries, and macOS SDK headers needed for CGO
// cross-compilation. Tool binaries are extracted from official OCI images
// via [dagger.Container.File] rather than compiled from source, giving
// faster builds, smaller Go build cache, and automatic platform matching.
func (m *Ci) releaserBase() *dagger.Container {
	return ensureGitRepo(dag.Container().
		From("golang:"+goVersion).
		// Install Zig for CGO cross-compilation.
		WithExec([]string{
			"sh", "-c",
			"apt-get update && apt-get install -y xz-utils && " +
				"ZIG_ARCH=$(uname -m | sed 's/arm64/aarch64/') && " +
				"curl -fsSL https://ziglang.org/download/" + zigVersion +
				"/zig-${ZIG_ARCH}-linux-" + zigVersion + ".tar.xz | " +
				"tar -xJ -C /usr/local --strip-components=1 && " +
				"ln -sf /usr/local/zig /usr/local/bin/zig",
		}).
		// Install GoReleaser from its official OCI image.
		WithFile("/usr/local/bin/goreleaser",
			dag.Container().From("ghcr.io/goreleaser/goreleaser:"+goreleaserVersion).
				File("/usr/bin/goreleaser")).
		// Install cosign from its official OCI image.
		WithFile("/usr/local/bin/cosign",
			dag.Container().From("gcr.io/projectsigstore/cosign:"+cosignVersion).
				File("/ko-app/cosign")).
		// Install syft from its official OCI image.
		WithFile("/usr/local/bin/syft",
			dag.Container().From("ghcr.io/anchore/syft:"+syftVersion).
				File("/syft")).
		// Pre-download KCL Language Server for all target platforms so the
		// GoReleaser per-build hook can copy it instead of hitting the
		// GitHub API (which is rate-limited and uncacheable by Dagger).
		WithExec([]string{
			"sh", "-c",
			"for pair in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do " +
				"os=${pair%%/*} && arch=${pair##*/} && " +
				"mkdir -p /lsp/${os}/${arch} && " +
				"curl -fsSL https://github.com/kcl-lang/kcl/releases/download/" + kclLSPVersion +
				"/kclvm-" + kclLSPVersion + "-${os}-${arch}.tar.gz | " +
				"tar -xz --strip-components=2 -C /lsp/${os}/${arch} kclvm/bin/kcl-language-server; " +
				"done",
		}).
		// Mount source and caches.
		WithMountedDirectory("/src", m.Source).
		WithWorkdir("/src").
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", dag.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		// Mount macOS SDK headers for Darwin cross-compilation.
		WithMountedDirectory("/sdk/MacOSX.sdk",
			m.Source.Directory(".nixpkgs/vendor/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk")).
		WithEnvVariable("SDK_PATH", "/sdk/MacOSX.sdk").
		// Env vars used by GoReleaser ldflags and templates.
		WithEnvVariable("KCL_LSP_VERSION", kclLSPVersion).
		WithEnvVariable("BUILD_TAGS", "netgo").
		WithEnvVariable("HOSTNAME", "dagger").
		WithEnvVariable("USER", "dagger").
		// CC/CXX env vars for GoReleaser cross-compilation via Zig.
		WithEnvVariable("CC_LINUX_AMD64", "/src/hack/zig-gold-wrapper.sh -target x86_64-linux-gnu").
		WithEnvVariable("CC_LINUX_ARM64", "/src/hack/zig-gold-wrapper.sh -target aarch64-linux-gnu").
		WithEnvVariable("CC_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none "+macosSDKFlags).
		WithEnvVariable("CC_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags).
		WithEnvVariable("CXX_LINUX_AMD64", "/src/hack/zig-gold-wrapper.sh -target x86_64-linux-gnu").
		WithEnvVariable("CXX_LINUX_ARM64", "/src/hack/zig-gold-wrapper.sh -target aarch64-linux-gnu").
		WithEnvVariable("CXX_DARWIN_AMD64", "/src/hack/zig-macos-wrapper.sh -target x86_64-macos-none "+macosSDKFlags).
		WithEnvVariable("CXX_DARWIN_ARM64", "/src/hack/zig-macos-wrapper.sh -target aarch64-macos-none "+macosSDKFlags))
}

// sanitizeCacheKey replaces characters that are invalid in Dagger cache
// volume names with hyphens.
func sanitizeCacheKey(name string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(name)
}

// ensureGitInit ensures the container has a minimal .git directory at its
// working directory. This is sufficient for tools that only need to locate
// the repository root (e.g. [FindRepoRoot] checks for .git/HEAD) but do
// not inspect commit history or the index. Prefer [ensureGitRepo] when the
// tool requires committed files (e.g. GoReleaser dirty-tree detection).
func ensureGitInit(ctr *dagger.Container) *dagger.Container {
	return ctr.WithExec([]string{
		"sh", "-c",
		"if ! git rev-parse --git-dir >/dev/null 2>&1; then " +
			"rm -f .git && " +
			"git init -q; " +
			"fi",
	})
}

// ensureGitRepo ensures the container has a valid git repository at its
// working directory with all files staged and committed. When running from
// a git worktree, the .git file references a host path that doesn't exist
// in the container. In that case, a full git repository is initialized so
// that tools like GoReleaser that depend on committed files, dirty-tree
// detection, and version derivation continue to work.
func ensureGitRepo(ctr *dagger.Container) *dagger.Container {
	return ctr.WithExec([]string{
		"sh", "-c",
		"if ! git rev-parse --git-dir >/dev/null 2>&1; then " +
			"rm -f .git && " +
			"git init -q && " +
			"git remote add origin https://github.com/macropower/kclipper.git && " +
			"git add -A && " +
			"GIT_COMMITTER_DATE='2000-01-01T00:00:00+00:00' " +
			"git -c user.email=ci@dagger -c user.name=ci commit -q --allow-empty -m init " +
			"--date='2000-01-01T00:00:00+00:00'; " +
			"fi",
	})
}
