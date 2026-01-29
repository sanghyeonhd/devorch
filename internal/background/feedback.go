package background

import "sync/atomic"

// 라우터/스케줄러에 힌트를 주는 피드백 채널(최소)
type Feedback struct {
	// 최근 queue depth
	queueDepth int64
	// 최근 congestion score 0..100
	congestion int64
}

func (f *Feedback) SetQueueDepth(v int64) { atomic.StoreInt64(&f.queueDepth, v) }
func (f *Feedback) QueueDepth() int64     { return atomic.LoadInt64(&f.queueDepth) }

func (f *Feedback) SetCongestion(v int64) { atomic.StoreInt64(&f.congestion, v) }
func (f *Feedback) Congestion() int64     { return atomic.LoadInt64(&f.congestion) }
