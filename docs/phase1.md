좋아요. 그럼 “실제 구현 수준”으로 바로 착수할 수 있게 아래 3가지를 한 번에 드릴게요.

1. OkAON DB 스키마(0003_okAON_learning.sql) 초안


2. Go 코드 파일별 최소 구현 스켈레톤(CRUD + 집계 + 리워드 기록 + 정책 스냅샷)


3. Router ↔ Learning(Bandit) 연결 포인트(SelectArm / UpdateReward / Explain)




---

1) 0003_okAON_learning.sql (OkAON + Learning 핵심 테이블)

> 목적: “작업 완료 기준”으로 벤치/품질/비용/지연/성공 여부를 기록하고, 그 결과로 밴딧/정책이 업데이트되게 함



핵심 엔티티

workload_run: 작업 1회 실행 기록 (task 단위)

model_attempt: 해당 작업에서 시도된 모델/프로바이더들(재시도/폴백 포함)

quality_eval: 품질 평가 결과(자동/휴먼/규칙)

reward_event: 리워드 계산 결과(학습 입력)

bandit_state: 모델 선택 정책 상태(arm별 통계)

policy_snapshot: 정책 버전/롤백용 스냅샷


-- 0003_okAON_learning.sql

PRAGMA foreign_keys=ON;

-- 1) 작업 실행(세션/프로젝트/카테고리/에이전트 단위)
CREATE TABLE IF NOT EXISTS ok_workload_run (
  run_id            TEXT PRIMARY KEY,
  created_at        INTEGER NOT NULL,
  project_id        TEXT NOT NULL,
  session_id        TEXT,
  workspace_id      TEXT,
  user_id           TEXT,

  agent_type        TEXT,     -- oracle/explore/atlas...
  category          TEXT,     -- quick/ultrabrain/visual...
  task_type         TEXT,     -- code_edit/debug/review/search/doc...
  prompt_hash       TEXT,     -- PII 제거 후 해시
  context_fingerprint TEXT,   -- repo+os+arch+hw+tools 등

  success           INTEGER NOT NULL,  -- 0/1
  fail_reason       TEXT,
  output_bytes      INTEGER,
  tokens_in         INTEGER,
  tokens_out        INTEGER,
  cost_microusd     INTEGER,  -- 비용을 마이크로달러 등 정수로
  latency_ms        INTEGER,
  end_to_end_ms     INTEGER
);

CREATE INDEX IF NOT EXISTS idx_ok_workload_run_proj_time
ON ok_workload_run(project_id, created_at);

CREATE INDEX IF NOT EXISTS idx_ok_workload_run_fingerprint
ON ok_workload_run(context_fingerprint);

-- 2) 모델/프로바이더 시도 기록(폴백 체인 관측)
CREATE TABLE IF NOT EXISTS ok_model_attempt (
  attempt_id        TEXT PRIMARY KEY,
  run_id            TEXT NOT NULL REFERENCES ok_workload_run(run_id) ON DELETE CASCADE,
  created_at        INTEGER NOT NULL,

  provider          TEXT NOT NULL, -- openai/anthropic/google/ollama/...
  model             TEXT NOT NULL, -- gpt-5.2/claude.../llama3...
  endpoint_kind     TEXT NOT NULL, -- remote/local
  temperature       REAL,
  max_tokens        INTEGER,

  started_at        INTEGER,
  finished_at       INTEGER,
  latency_ms        INTEGER,
  streamed          INTEGER NOT NULL,

  success           INTEGER NOT NULL,
  error_code        TEXT,
  error_message     TEXT,

  tokens_in         INTEGER,
  tokens_out        INTEGER,
  cost_microusd     INTEGER
);

CREATE INDEX IF NOT EXISTS idx_ok_model_attempt_run
ON ok_model_attempt(run_id);

-- 3) 품질 평가(자동 + 규칙 + 휴먼 확장)
CREATE TABLE IF NOT EXISTS ok_quality_eval (
  eval_id           TEXT PRIMARY KEY,
  run_id            TEXT NOT NULL REFERENCES ok_workload_run(run_id) ON DELETE CASCADE,
  created_at        INTEGER NOT NULL,

  evaluator         TEXT NOT NULL, -- auto/heuristic/human/rules
  score             REAL NOT NULL, -- 0.0 ~ 1.0
  dimensions_json   TEXT,          -- {"correctness":0.8,"style":0.7,...}
  notes             TEXT
);

CREATE INDEX IF NOT EXISTS idx_ok_quality_eval_run
ON ok_quality_eval(run_id);

-- 4) 리워드(학습 입력)
CREATE TABLE IF NOT EXISTS ok_reward_event (
  reward_id         TEXT PRIMARY KEY,
  run_id            TEXT NOT NULL REFERENCES ok_workload_run(run_id) ON DELETE CASCADE,
  created_at        INTEGER NOT NULL,

  chosen_provider   TEXT NOT NULL,
  chosen_model      TEXT NOT NULL,
  policy_key        TEXT NOT NULL, -- bandit key (context key)
  reward            REAL NOT NULL, -- -1 ~ +1 (설계)
  reward_breakdown_json TEXT        -- {"quality":0.4,"latency":-0.1,"cost":-0.05,"success":0.2}
);

CREATE INDEX IF NOT EXISTS idx_ok_reward_policy_time
ON ok_reward_event(policy_key, created_at);

-- 5) 밴딧 상태(arm별 통계 저장)
CREATE TABLE IF NOT EXISTS ok_bandit_state (
  policy_key        TEXT NOT NULL,
  arm_key           TEXT NOT NULL, -- provider/model 조합(또는 model만)
  updated_at        INTEGER NOT NULL,

  pulls             INTEGER NOT NULL,
  successes         INTEGER NOT NULL,
  reward_sum        REAL NOT NULL,
  reward_sq_sum     REAL NOT NULL, -- 분산 추정용
  last_reward       REAL,

  PRIMARY KEY(policy_key, arm_key)
);

CREATE INDEX IF NOT EXISTS idx_ok_bandit_state_updated
ON ok_bandit_state(updated_at);

-- 6) 정책 스냅샷(롤백/배포/비교)
CREATE TABLE IF NOT EXISTS ok_policy_snapshot (
  snapshot_id       TEXT PRIMARY KEY,
  created_at        INTEGER NOT NULL,

  project_id        TEXT,
  policy_key_prefix TEXT,
  algorithm         TEXT NOT NULL, -- thompson/ucb/eps...
  payload_json      TEXT NOT NULL, -- bandit state dump or params
  notes             TEXT
);

CREATE INDEX IF NOT EXISTS idx_ok_policy_snapshot_time
ON ok_policy_snapshot(created_at);


---

2) Go 코드 파일 “누락 없이” 최소 구현 스켈레톤

(A) internal/okAON/models.go

DB row 모델 + 공통 타입(ProviderModelKey, PolicyKey)


필수 구조체

WorkloadRun

ModelAttempt

QualityEval

RewardEvent

BanditArmState

PolicySnapshot



---

(B) internal/okAON/store.go

OkAON Store 인터페이스 (SQLite 외 확장 가능)


필수 메서드

InsertWorkloadRun

InsertModelAttempt

InsertQualityEval

InsertRewardEvent

UpsertBanditState

GetBanditState(policyKey)

GetRecentRuns(filter)

AggregateStats(filter)

SavePolicySnapshot / LoadPolicySnapshot



---

(C) internal/okAON/sqlite/insert.go

트랜잭션 기반 Insert (run + attempts + eval + reward를 한 묶음으로)


권장 패턴

BeginTx()

insert run

insert attempts

insert eval

insert reward

Commit



---

(D) internal/learning/bandit/thompson.go

Thompson Sampling: SelectArm(arms) / Update(arm, reward)


저장 형태

DB에 ok_bandit_state가 source of truth

러너는 요청 시 state를 로드하고 업데이트 반영



---

(E) internal/router/learning_adapter.go

Router 점수 계산에 “학습 보정치” 추가


핵심 함수

LearningBonus(policyKey, armKey) float64

ExplainLearning(policyKey, candidates)



---

3) Router ↔ Learning 실제 연결 흐름 (개발 순서 그대로)

Step 1) Router가 선택할 때

1. policyKey 생성 (context fingerprint + task_type + category + agent_type)


2. 후보 모델 리스트(가용성/권한/필터 통과) 뽑음


3. 기존 score.go 점수 계산


4. learning_adapter.go에서 보정치(Exploration 포함) 더함


5. 최종 선택



policyKey 예시

policyKey = sha256(
  project_id +
  os + arch + hw_tier +
  task_type + category + agent_type +
  toolchain_versions
)


---

Step 2) 작업 완료 시(“완료 기준”)

1. bench/recorder.go가 run 통계를 정리


2. bench/quality_eval.go가 품질 점수(0~1) 산출


3. bench/reward_emit.go가 reward(-1~+1) 계산


4. okAON에 run/attempt/eval/reward 저장


5. learning/learner.go가 bandit.Update(policyKey, armKey, reward) 수행




---

4) “리워드 설계” 기본값(바로 적용 가능한 형태)

> 사용자가 말한 “로컬에서 돌렸을 때의 실측 벤치”를 반영하려면, reward는 아래처럼 단순한 선형 조합이 가장 안전합니다.



success: +0.35 (실패면 -0.35)

quality: +0.45 * qualityScore

latency: -0.10 * normalize(latency)

cost: -0.10 * normalize(cost)


정규화는 프로젝트/머신별로 분산이 다르니

최근 N=200개 run을 기준으로 p50/p90을 잡아 상대값으로 변환



---

5) 다음 메시지에서 바로 “파일 내용”까지 내려드릴 수 있는 목록

원하시면 다음 턴에, 아래를 실제 Go 코드로 그대로 내려드릴게요(복붙 시작 가능):

internal/okAON/models.go 전체

internal/okAON/store.go 인터페이스 + filter 타입

internal/okAON/sqlite/insert.go / query.go / stats.go

internal/learning/bandit/thompson.go (DB-backed)

internal/router/learning_adapter.go (bonus + explain)

internal/bench/reward_emit.go (reward 계산)


원하는 방식만 고르세요(질문 아님, 그냥 선택지):

A안(추천): OkAON + Thompson만 먼저 붙이고 Router에 bonus 1줄로 연결

B안: OkAON + Thompson + UCB까지 같이(알고리즘 스위치 가능)


다음 답변에서는 A안 기준으로 “실제 코드 파일 5~7개”를 통째로 제공하겠습니다.