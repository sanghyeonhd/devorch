package streaming

// StreamCollector: streaming 동안 출력량을 누적(토큰 추정/바이트)
type StreamCollector struct {
	OutputBytes int64
	OutputText  int64 // rune 카운트(대략)
}

func (c *StreamCollector) AddText(s string) {
	c.OutputBytes += int64(len([]byte(s)))
	c.OutputText += int64(len([]rune(s)))
}
