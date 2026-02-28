package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"dagger/kclipper/internal/dagger"

	"golang.org/x/sync/errgroup"
)

// ReleaseReport captures the results of a release operation including
// image digests, artifact checksums, and a human-readable summary.
// Create instances via [Kclipper.Release].
type ReleaseReport struct {
	// Dist directory containing release artifacts.
	Dist *dagger.Directory
	// Tag is the version tag that was released (e.g. "v1.2.3").
	Tag string
	// ImageDigests contains published image digest references
	// (e.g. "registry/image:tag@sha256:hex"), one per tag published.
	ImageDigests []string
	// UniqueDigestCount is the number of unique image digests.
	// Tags may share a manifest, so this can be less than [TagCount].
	UniqueDigestCount int
	// TagCount is the number of tags published.
	TagCount int
}

// Summary returns a Markdown summary of the release suitable for
// $GITHUB_STEP_SUMMARY.
func (r *ReleaseReport) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Release Summary\n\n")
	if r.Tag != "" {
		fmt.Fprintf(&b, "- **Version:** `%s`\n", r.Tag)
	}
	fmt.Fprintf(&b, "- **Tags published:** %d\n", r.TagCount)
	fmt.Fprintf(&b, "- **Unique image digests:** %d\n\n", r.UniqueDigestCount)

	if len(r.ImageDigests) > 0 {
		fmt.Fprintf(&b, "### Published Image Digests\n\n")
		fmt.Fprintf(&b, "| Tag Reference | Digest |\n")
		fmt.Fprintf(&b, "| --- | --- |\n")
		for _, ref := range r.ImageDigests {
			parts := strings.SplitN(ref, "@", 2)
			if len(parts) == 2 {
				fmt.Fprintf(&b, "| `%s` | `%s` |\n", parts[0], parts[1])
			} else {
				fmt.Fprintf(&b, "| `%s` | — |\n", ref)
			}
		}
	}

	return b.String()
}

// optSecretVariable returns a [dagger.WithContainerFunc] that conditionally
// adds a secret environment variable. If the secret is nil, the container
// is returned unchanged.
func optSecretVariable(name string, secret *dagger.Secret) dagger.WithContainerFunc {
	return func(ctr *dagger.Container) *dagger.Container {
		if secret != nil {
			return ctr.WithSecretVariable(name, secret)
		}
		return ctr
	}
}

// reKCLModVersion matches the version line in kcl.mod files.
var reKCLModVersion = regexp.MustCompile(`(?m)^version\s*=\s*"[^"]*"`)

// isPrerelease reports whether the version tag contains a pre-release
// identifier (e.g. "v1.0.0-rc.1", "v2.0.0-beta.1"). Detection checks for
// a hyphen in any of the first three dot-separated version components after
// stripping the "v" prefix. Build metadata (e.g. "v1.0.0+build.1") does not
// contain a hyphen and is correctly treated as a stable release.
func isPrerelease(tag string) bool {
	v := strings.TrimPrefix(tag, "v")
	for _, part := range strings.SplitN(v, ".", 3) {
		if strings.Contains(part, "-") {
			return true
		}
	}
	return false
}

// VersionTags returns the image tags derived from a version tag string.
// For example, "v1.2.3" yields ["latest", "v1.2.3", "v1", "v1.2"].
// Pre-release versions (e.g. "v1.0.0-rc.1") yield only the exact tag.
func (m *Kclipper) VersionTags(
	// Version tag (e.g. "v1.2.3").
	tag string,
) []string {
	if isPrerelease(tag) {
		return []string{tag}
	}

	v := strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(v, ".", 3)

	tags := []string{"latest", tag}
	if len(parts) >= 1 {
		tags = append(tags, "v"+parts[0])
	}
	if len(parts) >= 2 {
		tags = append(tags, "v"+parts[0]+"."+parts[1])
	}
	return tags
}

// FormatDigestChecksums converts publish output references to the
// checksums format expected by actions/attest-build-provenance. Each reference
// has the form "registry/image:tag@sha256:hex"; this function emits
// "hex  registry/image:tag" lines, deduplicating by digest.
func (m *Kclipper) FormatDigestChecksums(
	// Image references (e.g. "registry/image:tag@sha256:hex").
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
func (m *Kclipper) DeduplicateDigests(
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
func (m *Kclipper) RegistryHost(
	// Registry address (e.g. "ghcr.io/macropower/kclipper").
	registry string,
) string {
	return strings.SplitN(registry, "/", 2)[0]
}

// patchedModulesDir discovers KCL modules under modules/ and rewrites each
// kcl.mod file's version line to the given version. Returns the patched
// directory and the list of module names.
func (m *Kclipper) patchedModulesDir(ctx context.Context, version string) (*dagger.Directory, []string, error) {
	modulesDir := m.Source.Directory("modules")
	entries, err := modulesDir.Entries(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list modules: %w", err)
	}

	patched := modulesDir
	var names []string
	for _, entry := range entries {
		modFile := modulesDir.File(entry + "/kcl.mod")
		content, err := modFile.Contents(ctx)
		if err != nil {
			// Skip directories without kcl.mod.
			continue
		}
		content = reKCLModVersion.ReplaceAllString(content, `version = "`+version+`"`)
		patched = patched.WithNewFile(entry+"/kcl.mod", content)
		names = append(names, entry)
	}

	return patched, names, nil
}

// PublishKCLModules pushes all KCL modules under modules/ to the OCI
// registry with the version derived from the given tag. The kclipper binary
// itself is used as the KCL CLI (it wraps kcl-lang.io/cli and kcl-lang.io/kpm).
//
// Pre-release tags (e.g. "v1.0.0-rc.1") are skipped; only stable releases
// are published to the module registry.
//
// +cache="never"
func (m *Kclipper) PublishKCLModules(
	ctx context.Context,
	// Version tag (e.g. "v1.2.3"). The "v" prefix is stripped for the
	// module version.
	tag string,
	// Registry username for authentication.
	// +optional
	registryUsername string,
	// Registry password or token for authentication.
	// +optional
	registryPassword *dagger.Secret,
) error {
	if isPrerelease(tag) {
		return nil
	}

	version := strings.TrimPrefix(tag, "v")

	patched, names, err := m.patchedModulesDir(ctx, version)
	if err != nil {
		return err
	}

	ctr := runtimeBase("").
		WithFile("/usr/local/bin/kcl", m.Binary("")).
		WithMountedDirectory("/modules", patched).
		WithWorkdir("/modules")

	// Conditional registry login (matching cosign pattern).
	if registryPassword != nil {
		ctr = ctr.
			WithSecretVariable("REGISTRY_PASSWORD", registryPassword).
			WithExec([]string{
				"sh", "-c",
				"kcl registry login -u " + registryUsername + " -p \"$REGISTRY_PASSWORD\" " + m.RegistryHost(m.Registry),
			})
	}

	// Push each module sequentially.
	for _, name := range names {
		ctr = ctr.
			WithWorkdir("/modules/" + name).
			WithExec([]string{
				"kcl", "mod", "push",
				"oci://" + m.Registry + "/" + name,
			})
	}

	_, err = ctr.Sync(ctx)
	return err
}

// PublishImages builds multi-arch container images using Dagger's native
// Container API and publishes them to the registry.
//
// Stable releases are published with multiple tags: :latest, :vX.Y.Z, :vX,
// :vX.Y. Pre-release versions are published with only their exact tag.
//
// +cache="never"
func (m *Kclipper) PublishImages(
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

	variants, err := m.BuildImages(ctx, version, dist)
	if err != nil {
		return "", err
	}
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword, cosignKey, cosignPassword)
	if err != nil {
		return "", err
	}

	// Deduplicate digests for the summary (tags may share a manifest).
	unique := m.DeduplicateDigests(digests)
	return fmt.Sprintf("published %d tags (%d unique digests)\n%s", len(tags), len(unique), strings.Join(digests, "\n")), nil
}

// Release runs GoReleaser for binaries/archives/signing, then builds and
// publishes container images using Dagger-native Container.Publish().
// GoReleaser's Docker support is skipped entirely to avoid Docker-in-Docker.
//
// Returns a [ReleaseReport] containing the dist/ directory (with checksums.txt
// and digests.txt for attestation), published image digests, and a Markdown
// summary suitable for $GITHUB_STEP_SUMMARY.
//
// +cache="never"
func (m *Kclipper) Release(
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
) (*ReleaseReport, error) {
	ctr, err := m.releaserBase(ctx)
	if err != nil {
		return nil, err
	}
	ctr = ctr.WithSecretVariable("GITHUB_TOKEN", githubToken)

	// Conditionally add cosign secrets for GoReleaser binary signing.
	skipFlags := "docker"
	if cosignKey == nil {
		skipFlags = "docker,sign"
	}
	ctr = ctr.
		With(optSecretVariable("COSIGN_KEY", cosignKey)).
		With(optSecretVariable("COSIGN_PASSWORD", cosignPassword))

	// Conditionally add macOS signing secrets.
	ctr = ctr.
		With(optSecretVariable("MACOS_SIGN_P12", macosSignP12)).
		With(optSecretVariable("MACOS_SIGN_PASSWORD", macosSignPassword)).
		With(optSecretVariable("MACOS_NOTARY_KEY", macosNotaryKey)).
		With(optSecretVariable("MACOS_NOTARY_KEY_ID", macosNotaryKeyId)).
		With(optSecretVariable("MACOS_NOTARY_ISSUER_ID", macosNotaryIssuerId))

	// Run GoReleaser for binaries, archives, Homebrew, Nix (and signing
	// when cosignKey is provided). Docker is always skipped -- images are
	// published natively via Dagger below.
	dist := ctr.
		WithExec([]string{"goreleaser", "release", "--clean", "--skip=" + skipFlags}).
		Directory("/src/dist")

	// Derive image tags from the version tag.
	tags := m.VersionTags(tag)

	// Publish multi-arch container images via Dagger-native API.
	variants, err := runtimeImages(ctx, dist, tag)
	if err != nil {
		return nil, fmt.Errorf("build runtime images: %w", err)
	}
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword, cosignKey, cosignPassword)
	if err != nil {
		return nil, fmt.Errorf("publish images: %w", err)
	}

	// Write digests in checksums format for attest-build-provenance.
	if len(digests) > 0 {
		checksums := m.FormatDigestChecksums(digests)
		dist = dist.WithNewFile("digests.txt", checksums)
	}

	unique := m.DeduplicateDigests(digests)
	return &ReleaseReport{
		Dist:              dist,
		Tag:               tag,
		ImageDigests:      digests,
		UniqueDigestCount: len(unique),
		TagCount:          len(tags),
	}, nil
}

// publishImages publishes pre-built container image variants to the registry,
// optionally signing them with cosign. Returns the list of published digest
// references (one per tag, e.g. "registry/image:tag@sha256:hex").
func (m *Kclipper) publishImages(
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
			With(optSecretVariable("COSIGN_KEY", cosignKey)).
			With(optSecretVariable("COSIGN_PASSWORD", cosignPassword))
		if registryPassword != nil {
			cosignCtr = cosignCtr.WithRegistryAuth(m.RegistryHost(m.Registry), registryUsername, registryPassword)
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
