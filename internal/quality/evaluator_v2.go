package quality

import (
	"context"
	"strings"
	"time"

	"devorch/internal/signal"
)

type EvaluatorV2 struct {
	Signals SignalReader
}

func NewEvaluatorV2(signals SignalReader) *EvaluatorV2 {
	return &EvaluatorV2{Signals: signals}
}

// 8단계 핵심: "완료 기준" 기반 점수
// - 테스트 성공(가산), 테스트 실패(대감점)
// - tool 실패율/주요 tool 실패(감점)
// - 에러 없고, 테스트 통과하고, tool 에러 적으면 높은 점수
func (e *EvaluatorV2) Evaluate(ctx context.Context, req EvalRequest, latency time.Duration, totalTokens int) EvalResult {
	if req.Error != nil {
		return EvalResult{Success: false, Score: 0.0, Notes: "provider_error", Error: errString(req.Error)}
	}

	ans := strings.TrimSpace(req.Answer)
	if ans == "" {
		return EvalResult{Success: false, Score: 0.0, Notes: "empty_answer"}
	}

	score := 0.45 // base

	// 응답 품질(간단 휴리스틱)
	if len(ans) > 600 {
		score += 0.15
	}
	if strings.Count(strings.ToUpper(ans), "TODO") >= 3 {
		score -= 0.15
	}

	// latency/token 페널티
	if latency > 30*time.Second {
		score -= 0.08
	}
	if totalTokens > 8000 {
		score -= 0.05
	}

	// Signals: ToolRuns + TestRuns
	var toolRuns []signal.ToolRun
	var testRuns []signal.TestRun
	if e.Signals != nil {
		toolRuns, _ = e.Signals.RecentToolRuns(ctx, req.Workspace, req.SessionID, req.TaskID, 30)
		testRuns, _ = e.Signals.RecentTestRuns(ctx, req.Workspace, req.SessionID, req.TaskID, 10)
	}

	// 테스트 우선
	if len(testRuns) == 0 {
		// 테스트가 없으면: 보수적으로 약간 감점 (DoD 부족)
		score -= 0.07
	} else {
		latest := testRuns[0]
		if latest.Success {
			score += 0.25
		} else {
			score -= 0.35
		}
	}

	// tool 실패율 반영
	if len(toolRuns) > 0 {
		fail := 0
		for _, tr := range toolRuns {
			if !tr.Success {
				fail++
			}
		}
		failRate := float64(fail) / float64(len(toolRuns))
		score -= (failRate * 0.20)

		// 주요 도구 실패(예: patch/apply, edit, exec 등) 추가 감점
		for _, tr := range toolRuns {
			if tr.Success {
				continue
			}
			if strings.Contains(tr.ToolName, "apply_patch") || strings.Contains(tr.ToolName, "edit") {
				score -= 0.08
				break
			}
		}
	}

	// success 판단: 점수 기반
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	success := score >= 0.55
	notes := "dod_signals"
	return EvalResult{Success: success, Score: score, Notes: notes}
}
