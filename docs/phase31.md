좋습니다. 31단계 풀코드는 “실측 Quality Gate + Reward Shaping(품질/지연/비용 반영)”을 완성합니다.

> ✅ 31단계 결과



bench/quality_eval.go 실제 구현

bench/recorder.go → quality_score 산출

background.finish()에서 reward = f(quality, latency, cost) 로 계산

OkAON에 quality_score + reward 실수값 저장

Learning 엔진이 연속값 보상으로 업데이트



---

A) 31단계 변경 파일 목록

신규

internal/bench/quality_eval.go

internal/bench/cost_model.go

internal/bench/reward_model.go


수정

internal/bench/recorder.go

internal/background/manager.go

internal/learning/learner.go (연속 reward 지원 확인/보완)

internal/okAON/sqlite/task_runs.go (이미 컬럼 있으므로 로직만 확인)



---

1) 품질 평가기 (Quality Gate)

1-1) internal/bench/quality_eval.go (신규)

package bench

import (
	"math"
	"strings"
)

type QualityInput struct {
	Prompt string
	Output string
	LatencyMs int64
}

type QualityResult struct {
	Score float64 // 0.0 ~ 1.0
	Notes []string
}

// 매우 경량의 로컬 품질 휴리스틱 (31단계)
// - 후속 단계에서 LLM-as-judge / task-type별 evaluator로 교체 가능
func EvaluateQuality(in QualityInput) QualityResult {
	var notes []string

	if strings.TrimSpace(in.Output) == "" {
		return QualityResult{Score: 0.0, Notes: []string{"empty_output"}}
	}

	length := len(in.Output)
	score := 0.5

	// 너무 짧으면 감점
	if length < 50 {
		score -= 0.2
		notes = append(notes, "too_short")
	}

	// 코드/구조 힌트 보너스
	if strings.Contains(in.Output, "```") || strings.Contains(in.Output, "package ") {
		score += 0.1
	}

	// 에러/사과 패턴 감점
	lower := strings.ToLower(in.Output)
	if strings.Contains(lower, "sorry") || strings.Contains(lower, "cannot") {
		score -= 0.1
		notes = append(notes, "apology_or_refusal")
	}

	// 지연 페널티 (5초 초과 시 선형 감점)
	if in.LatencyMs > 5000 {
		penalty := math.Min(0.3, float64(in.LatencyMs-5000)/20000.0)
		score -= penalty
		notes = append(notes, "slow_response")
	}

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return QualityResult{Score: score, Notes: notes}
}


---

2) 비용 모델 (Cost Model)

2-1) internal/bench/cost_model.go (신규)

package bench

// 단순 토큰 기반 비용 추정 (31단계)
// - 실제 API 단가 테이블은 32단계에서 provider별로 세분화
type CostInput struct {
	Provider string
	Model    string
	TokensIn  int64
	TokensOut int64
}

func EstimateCostUSD(in CostInput) float64 {
	// 기본 단가 (아주 러프)
	// remote는 비싸게, local은 0
	switch in.Provider {
	case "ollama", "local":
		return 0.0
	}

	// 기본: 1K 토큰당 $0.002 가정
	total := float64(in.TokensIn + in.TokensOut)
	return (total / 1000.0) * 0.002
}


---

3) Reward 모델 (Reward Shaping)

3-1) internal/bench/reward_model.go (신규)

package bench

import "math"

type RewardInput struct {
	QualityScore float64
	LatencyMs    int64
	CostUSD      float64
	Success      bool
}

// Reward = 품질(가중) - 지연 패널티 - 비용 패널티
func ComputeReward(in RewardInput) float64 {
	if !in.Success {
		return -1.0
	}

	// 품질 가중
	r := in.QualityScore * 2.0 // [0..2]

	// 지연 패널티 (10초 기준)
	if in.LatencyMs > 10000 {
		r -= math.Min(1.0, float64(in.LatencyMs-10000)/20000.0)
	}

	// 비용 패널티 (1달러 = -1)
	r -= in.CostUSD

	// 바운드
	if r < -2 {
		r = -2
	}
	if r > 2 {
		r = 2
	}
	return r
}


---

4) Recorder: quality_score 계산 연결

4-1) internal/bench/recorder.go (수정본)

package bench

type Recorder struct{}

func NewRecorder() *Recorder { return &Recorder{} }

func (r *Recorder) Evaluate(prompt, output string, latencyMs int64) (score float64, notes []string) {
	res := EvaluateQuality(QualityInput{
		Prompt: prompt,
		Output: output,
		LatencyMs: latencyMs,
	})
	return res.Score, res.Notes
}


---

5) Background Manager: reward + quality_score 실제 반영

5-1) internal/background/manager.go (31단계 핵심 수정 부분만 발췌)

> finish() 함수 내부 교체



import (
	"devorch/internal/bench"
)

// ...

func (m *Manager) finish(id string, start time.Time, out string, tin, tout int64, err error) error {
	end := time.Now()
	lat := end.Sub(start).Milliseconds()

	var status Status
	var errMsg string
	success := false
	if err != nil {
		status = StatusFailed
		errMsg = err.Error()
	} else {
		status = StatusSucceeded
		success = true
	}

	// ✅ 31단계: quality eval
	var qualityScore float64
	if success {
		q := bench.EvaluateQuality(bench.QualityInput{
			Prompt: "", // 필요시 저장된 prompt 사용 (store에서 가져와도 됨)
			Output: out,
			LatencyMs: lat,
		})
		qualityScore = q.Score
	}

	// ✅ 비용 추정
	cost := bench.EstimateCostUSD(bench.CostInput{
		Provider: m.store.MustGet(id).Provider,
		Model:    m.store.MustGet(id).Model,
		TokensIn: tin,
		TokensOut: tout,
	})

	// ✅ Reward shaping
	reward := bench.ComputeReward(bench.RewardInput{
		QualityScore: qualityScore,
		LatencyMs: lat,
		CostUSD: cost,
		Success: success,
	})

	_ = m.store.Update(id, func(t *Task) error {
		if t.Status == StatusCanceled {
			return nil
		}
		t.Status = status
		t.EndedAt = &end
		t.LatencyMs = lat
		t.TokensIn = tin
		t.TokensOut = tout
		t.Output = out
		t.Error = errMsg
		t.Reward = reward
		t.QualityScore = &qualityScore
		return nil
	})

	// OkAON 반영
	if m.ok != nil {
		tr := oksql.TaskRun{
			ID: id,
			FinishedAtUnix: sqlNullInt64(end.Unix()),
			Status: string(status),
			ErrorMessage: nullStr(errMsg),
			OutputText:   nullStr(out),
			LatencyMs:    sqlNullInt64(lat),
			TokensIn:     sqlNullInt64(tin),
			TokensOut:    sqlNullInt64(tout),
			QualityScore: sqlNullFloat64(qualityScore),
			Reward:       sqlNullFloat64(reward),
			EnvFingerprint: nullStr(m.envFP),
		}
		_ = m.ok.MarkFinished(context.Background(), tr)
	}

	// Learner 연속 보상
	if m.learner != nil {
		tt, _ := m.store.Get(id)
		if tt != nil {
			qs := qualityScore
			_ = m.learner.Observe(context.Background(), learning.Observation{
				TaskID: id,
				Category: tt.Category,
				SubagentType: tt.SubagentType,
				Provider: tt.Provider,
				Model: tt.Model,
				Success: success,
				Reward: reward,
				LatencyMs: lat,
				QualityScore: &qs,
				PromptHash: tt.PromptHash,
			})
		}
	}

	return nil
}


---

6) Learner: 연속 Reward 허용 확인

6-1) internal/learning/learner.go (필요시 보완)

package learning

type Observation struct {
	TaskID string
	Category string
	SubagentType string
	Provider string
	Model string

	Success bool
	Reward float64       // ✅ 연속값
	LatencyMs int64
	QualityScore *float64
	PromptHash string
}

// Learner 인터페이스는 Reward를 그대로 사용하도록 이미 설계되어 있어야 함

(이미 reward가 float64면 수정 불필요)


---

7) 31단계 검증 SQL

SELECT
  provider, model,
  AVG(quality_score) AS avg_q,
  AVG(reward) AS avg_r,
  AVG(latency_ms) AS avg_lat
FROM ok_aon_task_runs
WHERE finished_at_unix IS NOT NULL
GROUP BY provider, model
ORDER BY avg_r DESC;

→ 이제 Router + Bandit + Learning이
“성공/실패”가 아니라 품질/지연/비용까지 반영한 실수 reward로 학습합니다.


---

다음 32단계 (권장)

31단계로 “자기개선 엔진”이 완성되었습니다.
32단계에서는:

Provider별 실제 단가 테이블

latency/quality를 Router score에 직접 반영

“환경(env_fingerprint) × 모델”별 best-arm 캐시


로 현실적인 자동 최적화를 완성합니다.

원하면 바로 32단계 풀코드로 이어가겠습니다.