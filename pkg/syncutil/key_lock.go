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
// https://github.com/argoproj/pkg/blob/65f2d4777bfdabf8a3d649d705786567322cfa50/sync/key_lock.go

package syncutil

import "sync"

type KeyLock struct {
	locks map[string]*sync.RWMutex
	guard sync.RWMutex
}

func NewKeyLock() *KeyLock {
	return &KeyLock{
		guard: sync.RWMutex{},
		locks: map[string]*sync.RWMutex{},
	}
}

func (l *KeyLock) getLock(key string) *sync.RWMutex {
	l.guard.RLock()
	if lock, ok := l.locks[key]; ok {
		l.guard.RUnlock()

		return lock
	}

	l.guard.RUnlock()
	l.guard.Lock()

	if lock, ok := l.locks[key]; ok {
		l.guard.Unlock()

		return lock
	}

	lock := &sync.RWMutex{}
	l.locks[key] = lock
	l.guard.Unlock()

	return lock
}

func (l *KeyLock) Lock(key string) {
	l.getLock(key).Lock()
}

func (l *KeyLock) Unlock(key string) {
	l.getLock(key).Unlock()
}

func (l *KeyLock) RLock(key string) {
	l.getLock(key).RLock()
}

func (l *KeyLock) RUnlock(key string) {
	l.getLock(key).RUnlock()
}
