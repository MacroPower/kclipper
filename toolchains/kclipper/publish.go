package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/kclipper/internal/dagger"

	"golang.org/x/sync/errgroup"
)

// VersionTags returns the image tags derived from a version tag string.
// For example, "v1.2.3" yields ["latest", "v1.2.3", "v1", "v1.2"].
func (m *Kclipper) VersionTags(
	// Version tag (e.g. "v1.2.3").
	tag string,
) []string {
	v := strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(v, ".", 3)

	// Detect pre-release: any version component contains a hyphen
	// (e.g. "1.0.0-rc.1" -> third part is "0-rc.1").
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

	variants := m.BuildImages(version, dist)
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
// Returns the dist/ directory containing checksums.txt and digests.txt
// for attestation in the calling workflow.
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
) (*dagger.Directory, error) {
	ctr := m.releaserBase().
		WithSecretVariable("GITHUB_TOKEN", githubToken)

	// Conditionally add cosign secrets for GoReleaser binary signing.
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

	// Derive image tags from the version tag.
	tags := m.VersionTags(tag)

	// Publish multi-arch container images via Dagger-native API.
	variants := runtimeImages(dist, tag)
	digests, err := m.publishImages(ctx, variants, tags, registryUsername, registryPassword, cosignKey, cosignPassword)
	if err != nil {
		return nil, fmt.Errorf("publish images: %w", err)
	}

	// Write digests in checksums format for attest-build-provenance.
	if len(digests) > 0 {
		checksums := m.FormatDigestChecksums(digests)
		dist = dist.WithNewFile("digests.txt", checksums)
	}

	return dist, nil
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
