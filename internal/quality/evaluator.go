package quality

import (
	"math"
	"strings"
	"time"
)

type Evaluator struct {
	// 추후: repo conventions, test runner, tool results 등 주입
}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// MVP 스코어링:
// - error면 실패 + 낮은 점수
// - 답변 길이/구조(간단 휴리스틱)로 점수
// - "TODO만 잔뜩/공허한 답변" 페널티
func (e *Evaluator) Evaluate(req EvalRequest, latency time.Duration, totalTokens int) EvalResult {
	if req.Error != nil {
		return EvalResult{
			Success: false,
			Score:   0.0,
			Error:   errString(req.Error),
			Notes:   "provider_error",
		}
	}

	ans := strings.TrimSpace(req.Answer)
	if ans == "" {
		return EvalResult{Success: false, Score: 0.0, Notes: "empty_answer"}
	}

	score := 0.5

	// 길이 보정 (너무 짧으면 페널티, 적당하면 가산)
	n := float64(len(ans))
	score += clamp((math.Log1p(n/200.0) / 3.0), 0, 0.3)

	// "TODO" 비율이 높으면 페널티
	todo := strings.Count(strings.ToUpper(ans), "TODO")
	if todo >= 3 {
		score -= 0.2
	}

	// latency/token 페널티는 라우터에서 따로 반영하되, 품질에도 약간 반영
	if latency > 25*time.Second {
		score -= 0.1
	}
	if totalTokens > 8000 {
		score -= 0.05
	}

	score = clamp(score, 0, 1)
	return EvalResult{Success: true, Score: score, Notes: "heuristic"}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
