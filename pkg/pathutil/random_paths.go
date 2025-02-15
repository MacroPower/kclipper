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

package pathutil

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

type TempPaths interface {
	Add(key, value string)
	GetPath(key string) (string, error)
	GetPathIfExists(key string) string
	GetPaths() map[string]string
}

// RandomizedTempPaths allows generating and memoizing random paths, each path being mapped to a specific key.
type RandomizedTempPaths struct {
	paths map[string]string
	root  string
	lock  sync.RWMutex
}

func NewRandomizedTempPaths(root string) *RandomizedTempPaths {
	return &RandomizedTempPaths{
		root:  root,
		paths: map[string]string{},
	}
}

func (p *RandomizedTempPaths) Add(key, value string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.paths[key] = value
}

// GetPath generates a path for the given key or returns previously generated one.
func (p *RandomizedTempPaths) GetPath(key string) (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if val, ok := p.paths[key]; ok {
		return val, nil
	}

	uniqueID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuid: %w", err)
	}

	repoPath := filepath.Join(p.root, uniqueID.String())
	p.paths[key] = repoPath

	return repoPath, nil
}

// GetPathIfExists gets a path for the given key if it exists. Otherwise, returns an empty string.
func (p *RandomizedTempPaths) GetPathIfExists(key string) string {
	p.lock.RLock()
	defer p.lock.RUnlock()

	if val, ok := p.paths[key]; ok {
		return val
	}

	return ""
}

// GetPaths gets a copy of the map of paths.
func (p *RandomizedTempPaths) GetPaths() map[string]string {
	p.lock.RLock()
	defer p.lock.RUnlock()

	paths := map[string]string{}
	for k, v := range p.paths {
		paths[k] = v
	}

	return paths
}
