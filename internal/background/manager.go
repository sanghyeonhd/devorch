package background

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Task struct {
	ID          string
	Description string
	Run         func(ctx context.Context) error
	CreatedAt   time.Time
}

type Manager struct {
	feedback *Feedback

	queue    chan Task
	wg       sync.WaitGroup
	started  int32
	stopping int32

	// 관측용
	queueDepth int64

	// 동시 실행 제한
	concurrency int
}

func NewManager(feedback *Feedback, concurrency int) *Manager {
	if concurrency <= 0 {
		concurrency = 4
	}
	if feedback == nil {
		feedback = &Feedback{}
	}
	return &Manager{
		feedback:    feedback,
		queue:       make(chan Task, 1024),
		concurrency: concurrency,
	}
}

func (m *Manager) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&m.started, 0, 1) {
		return nil
	}

	// 워커 실행
	for i := 0; i < m.concurrency; i++ {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.worker(ctx)
		}()
	}

	// 관측 루프(혼잡/큐 뎁스)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				d := atomic.LoadInt64(&m.queueDepth)
				m.feedback.SetQueueDepth(d)
				m.feedback.SetCongestion(CongestionScore(d))
			}
		}
	}()

	return nil
}

func (m *Manager) Stop() {
	if atomic.CompareAndSwapInt32(&m.stopping, 0, 1) {
		close(m.queue)
	}
	m.wg.Wait()
}

func (m *Manager) Submit(t Task) error {
	if atomic.LoadInt32(&m.stopping) == 1 {
		return errors.New("background manager stopping")
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}

	atomic.AddInt64(&m.queueDepth, 1)
	select {
	case m.queue <- t:
		return nil
	default:
		atomic.AddInt64(&m.queueDepth, -1)
		return errors.New("background queue full")
	}
}

func (m *Manager) worker(ctx context.Context) {
	for task := range m.queue {
		atomic.AddInt64(&m.queueDepth, -1)

		// 실행
		_ = safeRun(ctx, task.Run)
	}
}

func safeRun(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered in background task: %v", r)
		}
	}()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}
