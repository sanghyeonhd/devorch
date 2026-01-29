package streaming

import "io"

// StreamWrapper: streaming response를 collector와 함께 래핑
type StreamWrapper struct {
	Source    io.Reader
	Collector *StreamCollector
}

func NewStreamWrapper(src io.Reader) *StreamWrapper {
	return &StreamWrapper{
		Source:    src,
		Collector: &StreamCollector{},
	}
}

func (w *StreamWrapper) Read(p []byte) (n int, err error) {
	n, err = w.Source.Read(p)
	if n > 0 && w.Collector != nil {
		w.Collector.AddText(string(p[:n]))
	}
	return n, err
}

// GetStats: 누적된 통계 반환
func (w *StreamWrapper) GetStats() (bytes int64, chars int64) {
	if w.Collector == nil {
		return 0, 0
	}
	return w.Collector.OutputBytes, w.Collector.OutputText
}
