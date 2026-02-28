// Security scans source dependencies and container images for known
// vulnerabilities using Trivy.
package main

import (
	"context"
	"dagger/security/internal/dagger"
)

const (
	defaultTrivyImage = "aquasec/trivy:0.68.2" // renovate: datasource=docker depName=aquasec/trivy
)

// Security scans source dependencies and container images for known
// vulnerabilities using Trivy. Create instances with [New].
type Security struct {
	// Source directory to scan for dependency vulnerabilities.
	Source *dagger.Directory

	// Trivy container image reference.
	Image string
}

// New creates a new [Security] module.
func New(
	// Project source directory.
	// +defaultPath="/"
	source *dagger.Directory,
	// Trivy container image.
	// +optional
	image string,
) *Security {
	if image == "" {
		image = defaultTrivyImage
	}
	return &Security{
		Source: source,
		Image:  image,
	}
}

// trivyBase returns a Trivy container with a locked cache volume
// at /root/.cache for reproducible scans.
func (m *Security) trivyBase() *dagger.Container {
	return dag.Container().
		From(m.Image).
		WithMountedCache(
			"/root/.cache",
			dag.CacheVolume("trivy-cache"),
			dagger.ContainerWithMountedCacheOpts{
				Sharing: dagger.CacheSharingModeLocked,
			},
		).
		WithWorkdir("/home/trivy")
}

// ScanSource scans source dependencies for known vulnerabilities.
// Reports CRITICAL and HIGH severity findings. Trivy auto-discovers
// a .trivyignore file in the scanned directory for CVE suppression.
func (m *Security) ScanSource(ctx context.Context) error {
	_, err := m.trivyBase().
		WithMountedDirectory(".", m.Source).
		WithExec([]string{
			"trivy", "fs",
			"--scanners=vuln",
			"--pkg-types=library",
			"--exit-code=1",
			"--severity=CRITICAL,HIGH",
			".",
		}).
		Sync(ctx)
	return err
}

// ScanImage scans a container image for known vulnerabilities in both
// OS packages and application libraries. Reports CRITICAL and HIGH
// severity findings.
func (m *Security) ScanImage(
	ctx context.Context,
	// Container to scan.
	target *dagger.Container,
) error {
	_, err := m.trivyBase().
		WithMountedFile("target.tar", target.AsTarball()).
		WithExec([]string{
			"trivy", "image",
			"--pkg-types=os,library",
			"--exit-code=1",
			"--severity=CRITICAL,HIGH",
			"--input=target.tar",
		}).
		Sync(ctx)
	return err
}
