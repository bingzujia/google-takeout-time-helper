package destlocker_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bingzujia/google-takeout-time-helper/internal/destlocker"
)

func TestLockUnlock(t *testing.T) {
	l := destlocker.New()
	unlock := l.Lock("path/A")
	unlock() // should not panic or deadlock
}

func TestDifferentPathsDoNotBlock(t *testing.T) {
	l := destlocker.New()
	done := make(chan struct{}, 2)

	var started atomic.Int32
	go func() {
		u := l.Lock("path/A")
		started.Add(1)
		defer u()
		done <- struct{}{}
	}()
	go func() {
		u := l.Lock("path/B")
		started.Add(1)
		defer u()
		done <- struct{}{}
	}()

	<-done
	<-done
}

func TestSamePathIsSerialized(t *testing.T) {
	l := destlocker.New()
	var counter int
	var mu sync.Mutex

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			unlock := l.Lock("shared/path")
			defer unlock()
			// Critical section: increment and immediately read — no race if lock works.
			mu.Lock()
			counter++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if counter != goroutines {
		t.Errorf("counter = %d, want %d", counter, goroutines)
	}
}
