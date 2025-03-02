// Copyright 2017-2018 The Argo Authors
// Modifications Copyright 2024-2025 Jacob Colvin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"sigs.k8s.io/yaml"

	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
)

var (
	ErrInvalidYAML         = errors.New("invalid yaml")
	ErrInvalidKubeResource = errors.New("invalid kubernetes resource")
)

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	objs := []*unstructured.Unstructured{}

	ymls, err := SplitYAMLToString(yamlData)
	if err != nil {
		return nil, fmt.Errorf("split yaml to strings: %w", err)
	}

	for _, yml := range ymls {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(yml), u); err != nil {
			return objs, fmt.Errorf("%w: %w", ErrInvalidKubeResource, err)
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

			return objs, fmt.Errorf("%w: %w", ErrInvalidYAML, err)
		}

		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}

		objs = append(objs, string(ext.Raw))
	}

	return objs, nil
}
