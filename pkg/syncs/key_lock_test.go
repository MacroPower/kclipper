package syncs_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/macropower/kclipper/pkg/syncs"
)

func TestKeyLock(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		newLock func() *syncs.KeyLock
	}{
		"with constructor": {
			newLock: syncs.NewKeyLock,
		},
		"zero value": {
			newLock: func() *syncs.KeyLock { return &syncs.KeyLock{} },
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			t.Run("lock and unlock same key", func(t *testing.T) {
				t.Parallel()

				kl := tc.newLock()
				kl.Lock("a")
				kl.Unlock("a")
			})

			t.Run("independent keys do not block each other", func(t *testing.T) {
				t.Parallel()

				kl := tc.newLock()

				kl.Lock("a")

				// Locking a different key must not block.
				done := make(chan struct{})
				go func() {
					kl.Lock("b")
					close(done)
				}()

				<-done

				kl.Unlock("a")
				kl.Unlock("b")
			})

			t.Run("same key serializes access", func(t *testing.T) {
				t.Parallel()

				kl := tc.newLock()

				counter := 0

				const n = 100

				var wg sync.WaitGroup
				wg.Add(n)

				for range n {
					go func() {
						defer wg.Done()

						kl.Lock("key")
						defer kl.Unlock("key")

						counter++
					}()
				}

				wg.Wait()

				assert.Equal(t, n, counter)
			})

			t.Run("concurrent keys are independent", func(t *testing.T) {
				t.Parallel()

				kl := tc.newLock()

				counters := map[string]*int{
					"x": new(int),
					"y": new(int),
					"z": new(int),
				}

				const n = 50

				var wg sync.WaitGroup

				for key, ctr := range counters {
					wg.Add(n)

					for range n {
						go func() {
							defer wg.Done()

							kl.Lock(key)
							defer kl.Unlock(key)

							*ctr++
						}()
					}
				}

				wg.Wait()

				for key, ctr := range counters {
					assert.Equal(t, n, *ctr, "counter for key %q", key)
				}
			})
		})
	}
}

func TestKeyLock_ImplementsKeyLocker(t *testing.T) {
	t.Parallel()

	var (
		_ syncs.KeyLocker = (*syncs.KeyLock)(nil)
		_ syncs.KeyLocker = &syncs.KeyLock{}
	)
}
