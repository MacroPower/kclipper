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
	"errors"
	"fmt"

	"go.jacobcolvin.com/niceyaml"
)

var (
	// ErrInvalidYAML indicates the input was not valid YAML.
	ErrInvalidYAML = errors.New("invalid yaml")

	// ErrInvalidKubeResource indicates the YAML did not represent a valid Kubernetes resource.
	ErrInvalidKubeResource = errors.New("invalid kubernetes resource")
)

// SplitYAML splits a YAML file into [Object] values. Returns list of all resources
// found in the yaml. If an error occurs, returns resources that have been parsed so far too.
func SplitYAML(yamlData []byte) ([]Object, error) {
	var resources []Object

	src := niceyaml.NewSourceFromBytes(yamlData,
		niceyaml.WithErrorOptions(niceyaml.WithSourceLines(3)),
	)

	if src.IsEmpty() {
		return nil, nil
	}

	decoder, err := src.Decoder()
	if err != nil {
		return resources, fmt.Errorf("%w: %w", ErrInvalidYAML, err)
	}

	for _, doc := range decoder.Documents() {
		var raw any

		err = doc.Decode(&raw)
		if err != nil {
			return resources, fmt.Errorf("%w: %w", ErrInvalidYAML, src.WrapError(err))
		}

		if raw == nil {
			continue
		}

		m, ok := raw.(map[string]any)
		if !ok {
			return resources, fmt.Errorf("%w: expected mapping, got %T", ErrInvalidKubeResource, raw)
		}

		resources = append(resources, Object(m))
	}

	return resources, nil
}
