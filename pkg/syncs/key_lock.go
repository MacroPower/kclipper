package syncs

import "sync"

// KeyLocker provides per-key mutual exclusion.
// See [KeyLock] for an implementation.
type KeyLocker interface {
	Lock(key string)
	Unlock(key string)
}

// KeyLock is a per-key mutex that allows independent keys to be locked
// concurrently while serializing access to the same key. Create instances with
// [NewKeyLock], or use the zero value directly.
type KeyLock struct {
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}

// NewKeyLock creates a new [KeyLock].
func NewKeyLock() *KeyLock {
	return &KeyLock{
		locks: make(map[string]*sync.Mutex),
	}
}

func (kl *KeyLock) getLock(key string) *sync.Mutex {
	kl.mu.Lock()
	defer kl.mu.Unlock()

	if kl.locks == nil {
		kl.locks = make(map[string]*sync.Mutex)
	}

	l, ok := kl.locks[key]
	if !ok {
		l = &sync.Mutex{}
		kl.locks[key] = l
	}

	return l
}

// Lock acquires the mutex for the given key, blocking if it is already held.
func (kl *KeyLock) Lock(key string) {
	kl.getLock(key).Lock()
}

// Unlock releases the mutex for the given key.
func (kl *KeyLock) Unlock(key string) {
	kl.getLock(key).Unlock()
}
