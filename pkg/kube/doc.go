// Package kube provides types and utilities for working with Kubernetes
// resource manifests.
//
// The central type is [Object], a map-based representation of a Kubernetes
// resource that supports common field accessors and deep copying. Raw YAML
// manifests can be split into individual objects with [SplitYAML].
package kube
