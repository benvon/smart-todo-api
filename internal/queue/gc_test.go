package queue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type mockDLQPurger struct {
	purgeFunc func(ctx context.Context, retention time.Duration) (int, error)
}

func (m *mockDLQPurger) PurgeOlderThan(ctx context.Context, retention time.Duration) (int, error) {
	if m.purgeFunc != nil {
		return m.purgeFunc(ctx, retention)
	}
	return 0, nil
}

func TestGarbageCollector_Collect_NilPurger(t *testing.T) {
	t.Parallel()
	gc := NewGarbageCollector(nil, time.Minute, 24*time.Hour)
	err := gc.collect(context.Background())
	if err != nil {
		t.Errorf("collect with nil purger: %v", err)
	}
}

func TestGarbageCollector_Collect_MockPurger(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	mock := &mockDLQPurger{
		purgeFunc: func(ctx context.Context, retention time.Duration) (int, error) {
			called.Store(true)
			if retention != 24*time.Hour {
				return 0, errors.New("unexpected retention")
			}
			return 3, nil
		},
	}
	gc := NewGarbageCollector(mock, time.Minute, 24*time.Hour)
	err := gc.collect(context.Background())
	if err != nil {
		t.Errorf("collect: %v", err)
	}
	if !called.Load() {
		t.Error("PurgeOlderThan was not called")
	}
}

func TestGarbageCollector_Collect_PurgerError(t *testing.T) {
	t.Parallel()
	mock := &mockDLQPurger{
		purgeFunc: func(context.Context, time.Duration) (int, error) {
			return 0, errors.New("purge failed")
		},
	}
	gc := NewGarbageCollector(mock, time.Minute, time.Hour)
	err := gc.collect(context.Background())
	if err == nil {
		t.Error("expected error from collect")
	}
}

func TestGarbageCollector_Start_StopsOnContextCancel(t *testing.T) {
	t.Parallel()
	mock := &mockDLQPurger{purgeFunc: func(context.Context, time.Duration) (int, error) { return 0, nil }}
	gc := NewGarbageCollector(mock, 24*time.Hour, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := gc.Start(ctx)
	if err == nil {
		t.Error("expected context cancelled error")
	}
}
