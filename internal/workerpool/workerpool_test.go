package workerpool_test

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/bingzujia/g_photo_take_out_helper/internal/workerpool"
)

func TestRunProcessesAllJobs(t *testing.T) {
	jobs := []int{1, 2, 3, 4, 5}
	var count atomic.Int64

	err := workerpool.Run(jobs, 2, func(j int) error {
		count.Add(1)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := count.Load(); got != int64(len(jobs)) {
		t.Errorf("processed %d jobs, want %d", got, len(jobs))
	}
}

func TestRunReturnsFirstError(t *testing.T) {
	sentinel := errors.New("boom")
	jobs := []int{1, 2, 3}

	err := workerpool.Run(jobs, 1, func(j int) error {
		if j == 2 {
			return sentinel
		}
		return nil
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestRunNoJobsNoError(t *testing.T) {
	err := workerpool.Run([]string{}, 4, func(s string) error {
		return errors.New("should not be called")
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDefaultWorkers(t *testing.T) {
	n := workerpool.DefaultWorkers()
	if n < 1 || n > 8 {
		t.Errorf("DefaultWorkers() = %d, want 1..8", n)
	}
}
