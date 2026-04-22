package workerpool

import (
	"runtime"
	"sync"
)

// DefaultWorkers returns a sensible default concurrency level.
func DefaultWorkers() int {
	n := runtime.NumCPU()
	if n > 8 {
		return 8
	}
	return n
}

// Run distributes jobs across workers goroutines, calling fn for each job.
// It returns the first non-nil error encountered; remaining in-flight work
// completes before Run returns (errors are not cancelled).
func Run[J any](jobs []J, workers int, fn func(J) error) error {
	if workers <= 0 {
		workers = DefaultWorkers()
	}

	ch := make(chan J, len(jobs))
	for _, j := range jobs {
		ch <- j
	}
	close(ch)

	var (
		firstErr error
		errOnce  sync.Once
		wg       sync.WaitGroup
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if err := fn(j); err != nil {
					errOnce.Do(func() { firstErr = err })
				}
			}
		}()
	}

	wg.Wait()
	return firstErr
}
