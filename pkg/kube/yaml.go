// Copyright 2017-2018 The Argo Authors. Licensed under the Apache License 2.0.
// Source:
// https://github.com/argoproj/gitops-engine/blob/54992bf42431e71f71f11647e82105530e56305e/pkg/utils/kube/kube.go#L304-L346

package kube

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	objs := []*unstructured.Unstructured{}
	ymls, err := SplitYAMLToString(yamlData)
	if err != nil {
		return nil, err
	}
	for _, yml := range ymls {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(yml), u); err != nil {
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}

// SplitYAMLToString splits a YAML file into strings. Returns list of yamls
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAMLToString(yamlData []byte) ([]string, error) {
	// Similar way to what kubectl does
	// https://github.com/kubernetes/cli-runtime/blob/master/pkg/resource/visitor.go#L573-L600
	// Ideally k8s.io/cli-runtime/pkg/resource.Builder should be used instead of this method.
	// E.g. Builder does list unpacking and flattening and this code does not.
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []string
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		objs = append(objs, string(ext.Raw))
	}
	return objs, nil
}
