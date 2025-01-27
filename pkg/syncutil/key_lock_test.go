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
// https://github.com/argoproj/pkg/blob/65f2d4777bfdabf8a3d649d705786567322cfa50/sync/key_lock_test.go

package syncutil_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/MacroPower/kclipper/pkg/syncutil"
)

func TestLockLock(t *testing.T) {
	t.Parallel()

	l := syncutil.NewKeyLock()

	l.Lock("my-key")

	unlocked := false

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		l.Lock("my-key")
		unlocked = true
		wg.Done()
	}()

	assert.False(t, unlocked)

	l.Unlock("my-key")

	wg.Wait()

	assert.True(t, unlocked)

	l.Unlock("my-key")
}

func TestLockRLock(t *testing.T) {
	t.Parallel()

	l := syncutil.NewKeyLock()

	l.Lock("my-key")

	unlocked := false

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		l.RLock("my-key")
		unlocked = true
		wg.Done()
	}()

	assert.False(t, unlocked)

	l.Unlock("my-key")

	wg.Wait()

	assert.True(t, unlocked)

	l.RUnlock("my-key")
}

func TestRLockLock(t *testing.T) {
	t.Parallel()

	l := syncutil.NewKeyLock()

	l.RLock("my-key")

	unlocked := false

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		l.Lock("my-key")
		unlocked = true
		wg.Done()
	}()

	assert.False(t, unlocked)

	l.RUnlock("my-key")

	wg.Wait()

	assert.True(t, unlocked)

	l.Unlock("my-key")
}

func TestRLockRLock(t *testing.T) {
	t.Parallel()

	l := syncutil.NewKeyLock()

	l.RLock("my-key")

	unlocked := false

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		l.RLock("my-key")
		unlocked = true
		wg.Done()
	}()

	wg.Wait()

	assert.True(t, unlocked)

	l.RUnlock("my-key")
	l.RUnlock("my-key")
}
