package usage

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPipelineRetriesWatcherCreation(t *testing.T) {
	pipeline := NewPipeline(
		&coordinatorTestStore{},
		coordinatorTestIngestor(func(context.Context, int64) (IngestResult, error) {
			return IngestResult{}, nil
		}),
		[]string{t.TempDir()},
		CoordinatorConfig{Workers: 1},
	)
	pipeline.restartWait = time.Millisecond
	created := make(chan struct{})
	var calls atomic.Int64
	pipeline.newWatcher = func(context.Context, []string) (transcriptWatcher, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("temporary watcher failure")
		}
		select {
		case <-created:
		default:
			close(created)
		}
		return newCoordinatorTestWatcher(), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := pipeline.Start(ctx)
	waitForCoordinatorSignal(t, created, "pipeline did not retry watcher creation")
	cancel()
	waitForCoordinatorSignal(t, done, "pipeline did not stop")
	if got := calls.Load(); got < 2 {
		t.Fatalf("watcher creation calls = %d, want at least 2", got)
	}
}


type pipelineRootUpdateWatcher struct {
	*coordinatorTestWatcher
	rootsMu sync.Mutex
	roots   [][]string
	updated chan struct{}
}

func newPipelineRootUpdateWatcher() *pipelineRootUpdateWatcher {
	return &pipelineRootUpdateWatcher{
		coordinatorTestWatcher: newCoordinatorTestWatcher(),
		updated:                make(chan struct{}, 4),
	}
}

func (w *pipelineRootUpdateWatcher) SetRoots(_ context.Context, roots []string) error {
	w.rootsMu.Lock()
	w.roots = append(w.roots, append([]string(nil), roots...))
	w.rootsMu.Unlock()
	select {
	case w.updated <- struct{}{}:
	default:
	}
	return nil
}

func TestPipelineAppliesDynamicRootToActiveWatcher(t *testing.T) {
	staticRoot := t.TempDir()
	dynamicRoot := t.TempDir()
	pipeline := NewPipeline(
		&coordinatorTestStore{},
		coordinatorTestIngestor(func(context.Context, int64) (IngestResult, error) {
			return IngestResult{}, nil
		}),
		[]string{staticRoot},
		CoordinatorConfig{Workers: 1},
	)
	watcher := newPipelineRootUpdateWatcher()
	created := make(chan struct{})
	pipeline.newWatcher = func(_ context.Context, roots []string) (transcriptWatcher, error) {
		if len(roots) != 1 || filepath.Clean(roots[0]) != filepath.Clean(staticRoot) {
			t.Fatalf("initial roots = %v, want [%s]", roots, staticRoot)
		}
		select {
		case <-created:
		default:
			close(created)
		}
		return watcher, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := pipeline.Start(ctx)
	waitForCoordinatorSignal(t, created, "pipeline watcher was not created")
	pipeline.AddWatchRoot(dynamicRoot)
	waitForCoordinatorSignal(t, watcher.updated, "dynamic root was not applied to active watcher")

	watcher.rootsMu.Lock()
	got := append([]string(nil), watcher.roots[len(watcher.roots)-1]...)
	watcher.rootsMu.Unlock()
	if len(got) != 2 {
		t.Fatalf("updated roots = %v, want static + dynamic", got)
	}
	seen := map[string]bool{}
	for _, root := range got {
		seen[filepath.Clean(root)] = true
	}
	if !seen[filepath.Clean(staticRoot)] || !seen[filepath.Clean(dynamicRoot)] {
		t.Fatalf("updated roots = %v", got)
	}

	cancel()
	waitForCoordinatorSignal(t, done, "pipeline did not stop")
}
