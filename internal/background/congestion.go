package background

// queueDepth 기반 혼잡 점수 계산(0..100)
// 매우 단순: depth 0 => 0, depth 50 => 100
func CongestionScore(queueDepth int64) int64 {
	if queueDepth <= 0 {
		return 0
	}
	if queueDepth >= 50 {
		return 100
	}
	return queueDepth * 2
}
