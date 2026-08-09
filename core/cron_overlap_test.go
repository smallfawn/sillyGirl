package core

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestSkipIfStillRunningCronFunc(t *testing.T) {
	var baselineRuns atomic.Int32
	baselineStarted := make(chan struct{}, 2)
	baselineRelease := make(chan struct{})
	baselineJob := cron.FuncJob(func() {
		baselineRuns.Add(1)
		baselineStarted <- struct{}{}
		<-baselineRelease
	})
	baselineDone := make(chan struct{}, 2)
	for range 2 {
		go func() {
			baselineJob.Run()
			baselineDone <- struct{}{}
		}()
	}
	for range 2 {
		select {
		case <-baselineStarted:
		case <-time.After(time.Second):
			t.Fatal("baseline overlapping cron run did not start")
		}
	}
	if got := baselineRuns.Load(); got != 2 {
		t.Fatalf("baseline overlapping runs = %d, want 2", got)
	}
	t.Logf("baseline_plain_cron overlapping_runs=%d", baselineRuns.Load())
	close(baselineRelease)
	for range 2 {
		select {
		case <-baselineDone:
		case <-time.After(time.Second):
			t.Fatal("baseline cron run did not finish")
		}
	}

	var runs atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	job := skipIfStillRunningCronFunc(func() {
		runs.Add(1)
		started <- struct{}{}
		<-release
	})

	firstDone := make(chan struct{})
	go func() {
		job.Run()
		close(firstDone)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first cron run did not start")
	}

	secondDone := make(chan struct{})
	go func() {
		job.Run()
		close(secondDone)
	}()
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("overlapping cron run was not skipped")
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("runs during overlap = %d, want 1", got)
	}
	t.Logf("modified_skip_if_running overlapping_runs=%d", runs.Load())

	close(release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first cron run did not finish")
	}

	job.Run()
	if got := runs.Load(); got != 2 {
		t.Fatalf("runs after previous completion = %d, want 2", got)
	}
}
