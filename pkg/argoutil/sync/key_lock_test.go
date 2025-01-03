package sync

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLockLock(t *testing.T) {
	t.Parallel()

	l := NewKeyLock()

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

	l := NewKeyLock()

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

	l := NewKeyLock()

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

	l := NewKeyLock()

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
