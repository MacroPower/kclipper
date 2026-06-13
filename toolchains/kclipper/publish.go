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
	pre, err := m.Goreleaser.IsPrerelease(ctx, tag)
	if err != nil {
		return err
	}
	if pre {
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
		host, err := m.Goreleaser.RegistryHost(ctx, m.Registry)
		if err != nil {
			return err
		}
		ctr = ctr.
			WithSecretVariable("REGISTRY_PASSWORD", registryPassword).
			WithExec([]string{
				"sh", "-c",
				"kcl registry login -u " + registryUsername + " -p \"$REGISTRY_PASSWORD\" " + host,
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
	// OIDC token request URL for keyless Sigstore signing. In GitHub Actions
	// this is the ACTIONS_ID_TOKEN_REQUEST_URL environment variable. When
	// provided along with oidcRequestToken, published images are signed
	// using Sigstore keyless verification (Fulcio + Rekor).
	// +optional
	oidcRequestURL string,
	// Bearer token for the OIDC token request. In GitHub Actions this is the
	// ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable.
	// +optional
	oidcRequestToken *dagger.Secret,
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
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword)
	if err != nil {
		return "", err
	}

	if err := m.signImages(ctx, digests, registryUsername, registryPassword, oidcRequestURL, oidcRequestToken); err != nil {
		return "", err
	}

	// Deduplicate digests for the summary (tags may share a manifest).
	unique, err := m.Goreleaser.DeduplicateDigests(ctx, digests)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("published %d tags (%d unique digests)\n%s", len(tags), len(unique), strings.Join(digests, "\n")), nil
}

// Release runs GoReleaser for binaries/archives/signing, then builds and
// publishes container images using Dagger-native Container.Publish().
// GoReleaser's Docker support is skipped entirely to avoid Docker-in-Docker.
//
// Both binary archives and container images are signed using Sigstore keyless
// verification when OIDC request credentials are provided. Cosign's built-in
// GitHub Actions provider fetches fresh tokens on demand, avoiding expiry
// issues with pre-fetched tokens.
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
	// OIDC token request URL for keyless Sigstore signing. In GitHub Actions
	// this is the ACTIONS_ID_TOKEN_REQUEST_URL environment variable.
	// +optional
	oidcRequestURL string,
	// Bearer token for the OIDC token request. In GitHub Actions this is the
	// ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable.
	// +optional
	oidcRequestToken *dagger.Secret,
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

	// Conditionally forward OIDC credentials for GoReleaser blob signing.
	// Cosign (invoked by GoReleaser's signs section) detects
	// ACTIONS_ID_TOKEN_REQUEST_URL/TOKEN and fetches fresh OIDC tokens
	// on demand via its built-in GitHub Actions provider.
	skipFlags := "docker"
	if oidcRequestToken == nil {
		skipFlags = "docker,sign"
	}
	ctr = ctr.
		WithEnvVariable("ACTIONS_ID_TOKEN_REQUEST_URL", oidcRequestURL).
		With(optSecretVariable("ACTIONS_ID_TOKEN_REQUEST_TOKEN", oidcRequestToken))

	// Conditionally add macOS signing secrets.
	ctr = ctr.
		With(optSecretVariable("MACOS_SIGN_P12", macosSignP12)).
		With(optSecretVariable("MACOS_SIGN_PASSWORD", macosSignPassword)).
		With(optSecretVariable("MACOS_NOTARY_KEY", macosNotaryKey)).
		With(optSecretVariable("MACOS_NOTARY_KEY_ID", macosNotaryKeyId)).
		With(optSecretVariable("MACOS_NOTARY_ISSUER_ID", macosNotaryIssuerId))

	// Run GoReleaser for binaries, archives, Homebrew, Nix (and signing
	// when oidcRequestToken is provided). Docker is always skipped -- images
	// are published natively via Dagger below.
	dist := ctr.
		WithExec([]string{"goreleaser", "release", "--clean", "--skip=" + skipFlags}).
		Directory("/src/dist")

	// Derive image tags from the version tag.
	tags, err := m.Goreleaser.VersionTags(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("derive version tags: %w", err)
	}

	// Publish multi-arch container images via Dagger-native API.
	variants, err := runtimeImages(ctx, dist, tag)
	if err != nil {
		return nil, fmt.Errorf("build runtime images: %w", err)
	}
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword)
	if err != nil {
		return nil, fmt.Errorf("publish images: %w", err)
	}

	if err := m.signImages(ctx, digests, registryUsername, registryPassword, oidcRequestURL, oidcRequestToken); err != nil {
		return nil, err
	}

	// Write digests in checksums format for attest-build-provenance.
	if len(digests) > 0 {
		checksums, err := m.Goreleaser.FormatDigestChecksums(ctx, digests)
		if err != nil {
			return nil, fmt.Errorf("format digest checksums: %w", err)
		}
		dist = dist.WithNewFile("digests.txt", checksums)
	}

	unique, err := m.Goreleaser.DeduplicateDigests(ctx, digests)
	if err != nil {
		return nil, fmt.Errorf("deduplicate digests: %w", err)
	}
	return &ReleaseReport{
		Dist:              dist,
		Tag:               tag,
		ImageDigests:      digests,
		UniqueDigestCount: len(unique),
		TagCount:          len(tags),
	}, nil
}

// publishImages publishes pre-built container image variants to the registry.
// Returns the list of published digest references (one per tag,
// e.g. "registry/image:tag@sha256:hex").
func (m *Kclipper) publishImages(
	ctx context.Context,
	variants []*dagger.Container,
	tags []string,
	registryUsername string,
	registryPassword *dagger.Secret,
) ([]string, error) {
	// Publish multi-arch manifest for each tag concurrently.
	publisher := dag.Container()
	if registryPassword != nil {
		host, err := m.Goreleaser.RegistryHost(ctx, m.Registry)
		if err != nil {
			return nil, fmt.Errorf("resolve registry host: %w", err)
		}
		publisher = publisher.WithRegistryAuth(host, registryUsername, registryPassword)
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

	return digests, nil
}

// signImages signs published image digests using cosign keyless signing
// (Fulcio + Rekor) via the shared [Cosign] toolchain. Cosign's built-in
// GitHub Actions provider uses the request URL and token to fetch fresh OIDC
// tokens on demand, avoiding expiry issues. Digests are deduplicated before
// signing since multiple tags often share one manifest. Does nothing when
// oidcRequestToken is nil.
func (m *Kclipper) signImages(
	ctx context.Context,
	digests []string,
	registryUsername string,
	registryPassword *dagger.Secret,
	oidcRequestURL string,
	oidcRequestToken *dagger.Secret,
) error {
	if oidcRequestToken == nil {
		return nil
	}

	toSign, err := m.Goreleaser.DeduplicateDigests(ctx, digests)
	if err != nil {
		return fmt.Errorf("deduplicate digests: %w", err)
	}

	host := ""
	if registryPassword != nil {
		host, err = m.Goreleaser.RegistryHost(ctx, m.Registry)
		if err != nil {
			return fmt.Errorf("resolve registry host: %w", err)
		}
	}

	return m.Cosign.SignKeyless(ctx, toSign, oidcRequestURL, oidcRequestToken,
		dagger.CosignSignKeylessOpts{
			RegistryHost:     host,
			RegistryUsername: registryUsername,
			RegistryPassword: registryPassword,
		})
}
