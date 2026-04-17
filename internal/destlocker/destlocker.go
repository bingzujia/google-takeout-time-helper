package destlocker

import "sync"

// Locker provides per-path mutual exclusion. Multiple goroutines can hold
// locks on different paths simultaneously; only one goroutine at a time can
// hold the lock for a given path.
type Locker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// New returns a new Locker ready for use.
func New() *Locker {
	return &Locker{locks: make(map[string]*sync.Mutex)}
}

// Lock acquires the mutex for path and returns an unlock function. The caller
// must call the returned function to release the lock.
func (d *Locker) Lock(path string) func() {
	d.mu.Lock()
	lock, ok := d.locks[path]
	if !ok {
		lock = &sync.Mutex{}
		d.locks[path] = lock
	}
	d.mu.Unlock()

	lock.Lock()
	return lock.Unlock
}
