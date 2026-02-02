좋습니다. Step 12 = “모든 Provider(ollama/openai/anthropic/openrouter 등) 호출의 종료 시점을 100% 포착해서 Bench 자동 기록 + Scenario 분류까지 고정” 단계입니다.

이번 단계의 목표는 딱 3가지입니다.

1. LLM 호출이 어디서 일어나든(세션/툴/에이전트/백그라운드) 호출 종료 시점에 Observer.OnComplete()가 무조건 1번 실행되도록 만들기


2. Streaming 포함(SSE/stream chunks)에서도 종료 시점에 기록되도록 만들기


3. Scenario를 “chat / tool-call / code-edit / embedding …” 처럼 일관되게 넣어서, Step10/11의 라우터가 “실사용 벤치”를 학습할 수 있게 만들기



아래는 Step 12 풀코드입니다. (파일별)


---

Step 12 추가/수정 트리

devorch/
└─ internal/
   ├─ provider/
   │  ├─ provider.go                          # (MOD) call options에 Scenario/Observer/Telemetry 추가
   │  ├─ errors.go                            # (NEW) 에러 문자열 표준화
   │  ├─ metering/
   │  │  ├─ tokens.go                         # (NEW) token 추정/정리 헬퍼
   │  │  └─ latency.go                        # (NEW) latency 측정 헬퍼
   │  ├─ observer/
   │  │  ├─ observer.go                       # (Step11) 그대로 사용
   │  │  ├─ bench_recorder.go                 # (Step11) 그대로 사용
   │  │  └─ chain.go                          # (NEW) observer chain (multi)
   │  ├─ streaming/
   │  │  ├─ stream.go                         # (NEW) 공통 스트림 타입
   │  │  ├─ wrap.go                           # (NEW) 스트리밍 종료 포착 wrapper
   │  │  └─ collector.go                      # (NEW) output token/bytes 수집기
   │  ├─ instrument/
   │  │  ├─ instrument.go                     # (NEW) non-stream/stream 공통 계측 wrapper
   │  │  └─ scenario.go                       # (NEW) scenario 규칙/상수
   │  └─ registry.go                          # (MOD) provider 생성 시 observer 주입
   │
   ├─ session/
   │  ├─ llm.go                               # (NEW) session에서 provider 호출 시 Scenario="chat"
   │  └─ toolcall.go                          # (NEW) tool 실행에서 Scenario="tool-call"
   │
   └─ server/
      └─ routes/
         └─ provider.go                       # (MOD) 디버그 요청에도 scenario 전달(옵션)

> ⚠️ 주의: 당신 프로젝트에 이미 provider.go/registry.go/session/llm.ts 같은 파일이 있을 수 있습니다.
Step12는 “그 구조를 덮어쓰는 게 아니라”, “공통 wrapper(계측기)”를 끼워 넣는 방식이라서 기존 코드에 붙이기 쉽습니다.




---

1) Scenario 표준 (절대 흔들리지 않게 고정)

1.1 internal/provider/instrument/scenario.go

package instrument

// Scenario = 벤치 집계의 키가 되는 “작업 유형”
// 절대 마음대로 문자열 바꾸지 말 것 (DB 집계가 깨짐)
type Scenario string

const (
	ScenarioChat      Scenario = "chat"
	ScenarioToolCall  Scenario = "tool-call"
	ScenarioCodeEdit  Scenario = "code-edit"
	ScenarioCodeReview Scenario = "code-review"
	ScenarioEmbedding Scenario = "embedding"
	ScenarioRerank    Scenario = "rerank"
	ScenarioSummarize Scenario = "summarize"
	ScenarioOther     Scenario = "other"
)

func NormalizeScenario(s string) Scenario {
	switch Scenario(s) {
	case ScenarioChat, ScenarioToolCall, ScenarioCodeEdit, ScenarioCodeReview,
		ScenarioEmbedding, ScenarioRerank, ScenarioSummarize:
		return Scenario(s)
	}
	if s == "" {
		return ScenarioOther
	}
	return ScenarioOther
}


---

2) Provider 공통 CallOptions 확장 (Observer + Scenario + Machine 정보)

2.1 internal/provider/provider.go (MOD)

package provider

import (
	"context"

	"devorch/internal/provider/instrument"
	"devorch/internal/provider/observer"
)

// ChatMessage: 최소 공통 메시지 형태
type ChatMessage struct {
	Role    string
	Content string
}

// ChatRequest: 최소 공통 요청
type ChatRequest struct {
	Messages []ChatMessage
}

// ChatDelta: streaming chunk
type ChatDelta struct {
	Text string
}

// ChatResponse: non-stream 응답
type ChatResponse struct {
	Text string

	InputTokens  int64
	OutputTokens int64
}

// Stream: provider 공통 스트림 인터페이스
type Stream[T any] interface {
	Recv() (T, error) // delta 수신
	Close() error     // 사용자가 중단할 때
}

// CallOptions: Step12의 핵심 - scenario/observer를 여기서 표준화
type CallOptions struct {
	Scenario instrument.Scenario

	OS                 string
	Arch               string
	MachineFingerprint string

	Observer observer.Observer // nil 가능

	// provider별 raw metadata
	Metadata map[string]any
}

// Provider: 각 vendor/로컬 런타임 구현체가 이 인터페이스를 구현
type Provider interface {
	Name() string

	Chat(ctx context.Context, req ChatRequest, opt CallOptions) (ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, opt CallOptions) (Stream[ChatDelta], error)
}


---

3) Observer 체인 (여러 observer를 동시에)

3.1 internal/provider/observer/chain.go

package observer

import "context"

type Chain struct {
	Items []Observer
}

func (c Chain) OnComplete(ctx context.Context, call ProviderCall) error {
	for _, o := range c.Items {
		if o == nil {
			continue
		}
		_ = o.OnComplete(ctx, call)
	}
	return nil
}


---

4) 에러 문자열 표준화 (DB에 저장할 error는 “짧고 일정”해야 함)

4.1 internal/provider/errors.go

package provider

import "strings"

func ShortError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		s = s[:240]
	}
	return s
}


---

5) Streaming 공통 구현 + 종료 포착 Wrapper

5.1 internal/provider/streaming/stream.go

package streaming

import "io"

// EndOfStream: 스트림 종료를 표현하기 위해 io.EOF를 그대로 사용
var EndOfStream = io.EOF

5.2 internal/provider/streaming/collector.go

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

5.3 internal/provider/streaming/wrap.go

package streaming

import (
	"context"
	"io"
	"time"

	"devorch/internal/provider"
	"devorch/internal/provider/instrument"
	"devorch/internal/provider/observer"
)

// WrapStream: provider.Stream을 감싸서 “종료 시점”에 observer 기록을 보장
type WrapStream struct {
	Inner provider.Stream[provider.ChatDelta]

	Ctx context.Context

	StartedAt time.Time
	Provider  string
	Model     string
	Scenario  instrument.Scenario

	OS                 string
	Arch               string
	MachineFingerprint string

	InputTokens  int64 // 가능한 경우 채움
	OutputTokens int64 // 추정치 (collector 기반)

	Success bool
	Err     string

	Collector *StreamCollector

	Observer observer.Observer

	done bool
}

func (w *WrapStream) Recv() (provider.ChatDelta, error) {
	d, err := w.Inner.Recv()
	if err == nil {
		if w.Collector != nil && d.Text != "" {
			w.Collector.AddText(d.Text)
		}
		return d, nil
	}

	// 종료 처리
	if err == io.EOF {
		w.Success = true
		w.finish()
		return provider.ChatDelta{}, io.EOF
	}

	w.Success = false
	w.Err = provider.ShortError(err)
	w.finish()
	return provider.ChatDelta{}, err
}

func (w *WrapStream) Close() error {
	err := w.Inner.Close()
	// Close() 호출 시도도 종료로 본다(사용자 취소)
	if !w.done {
		if err != nil {
			w.Success = false
			w.Err = provider.ShortError(err)
		} else {
			// 사용자 close는 success로 볼지 말지는 정책인데,
			// 일단 "false + canceled"가 더 분석에 유리함
			w.Success = false
			if w.Err == "" {
				w.Err = "canceled"
			}
		}
		w.finish()
	}
	return err
}

func (w *WrapStream) finish() {
	if w.done {
		return
	}
	w.done = true

	if w.Observer == nil {
		return
	}

	lat := time.Since(w.StartedAt).Milliseconds()
	var outTok *int64
	if w.Collector != nil {
		// 토큰은 provider별로 정확히 넣을 수 있으면 best.
		// 없으면 “문자 기반 추정치”로라도 남긴다.
		est := estimateTokensFromText(w.Collector.OutputText)
		outTok = &est
	}

	call := observer.ProviderCall{
		CreatedAt: time.Now(),

		Provider: w.Provider,
		Model:    w.Model,
		Scenario: string(w.Scenario),

		OS:                 w.OS,
		Arch:               w.Arch,
		MachineFingerprint: w.MachineFingerprint,

		InputTokens:  w.InputTokens,
		OutputTokens: derefI64(outTok),
		LatencyMs:    lat,

		Success: w.Success,
		Error:   w.Err,
		Metadata: map[string]any{
			"stream": true,
		},
	}

	_ = w.Observer.OnComplete(w.Ctx, call)
}

func estimateTokensFromText(runes int64) int64 {
	// 매우 러프: 한국어/영어 섞여도 “대략적”으로만
	if runes <= 0 {
		return 0
	}
	// 1 token ~= 3~4 chars 정도로 대충 잡음
	return runes / 4
}

func derefI64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}


---

6) Non-stream / Stream 공통 계측 Wrapper (핵심)

6.1 internal/provider/instrument/instrument.go

package instrument

import (
	"context"
	"time"

	"devorch/internal/provider"
	"devorch/internal/provider/observer"
	"devorch/internal/provider/streaming"
)

// InstrumentedProvider: 기존 provider를 감싸서 “종료 시점 기록”을 강제한다.
type InstrumentedProvider struct {
	Inner provider.Provider

	ProviderName string
	ModelName    string

	Observer observer.Observer
}

func Wrap(p provider.Provider, providerName, modelName string, obs observer.Observer) provider.Provider {
	return &InstrumentedProvider{
		Inner:        p,
		ProviderName: providerName,
		ModelName:    modelName,
		Observer:     obs,
	}
}

func (p *InstrumentedProvider) Name() string {
	if p.Inner != nil {
		return p.Inner.Name()
	}
	return p.ProviderName
}

func (p *InstrumentedProvider) Chat(ctx context.Context, req provider.ChatRequest, opt provider.CallOptions) (provider.ChatResponse, error) {
	start := time.Now()

	opt.Scenario = NormalizeScenario(string(opt.Scenario))
	obs := opt.Observer
	if obs == nil {
		obs = p.Observer
	}

	resp, err := p.Inner.Chat(ctx, req, opt)

	// 종료 기록
	if obs != nil {
		call := observer.ProviderCall{
			CreatedAt: time.Now(),

			Provider: p.ProviderName,
			Model:    p.ModelName,
			Scenario: string(opt.Scenario),

			OS:                 opt.OS,
			Arch:               opt.Arch,
			MachineFingerprint: opt.MachineFingerprint,

			InputTokens:  resp.InputTokens,
			OutputTokens: resp.OutputTokens,
			LatencyMs:    time.Since(start).Milliseconds(),

			Success: err == nil,
			Error:   provider.ShortError(err),
			Metadata: mergeMeta(opt.Metadata, map[string]any{
				"stream": false,
			}),
		}
		_ = obs.OnComplete(ctx, call)
	}

	return resp, err
}

func (p *InstrumentedProvider) ChatStream(ctx context.Context, req provider.ChatRequest, opt provider.CallOptions) (provider.Stream[provider.ChatDelta], error) {
	start := time.Now()

	opt.Scenario = NormalizeScenario(string(opt.Scenario))
	obs := opt.Observer
	if obs == nil {
		obs = p.Observer
	}

	st, err := p.Inner.ChatStream(ctx, req, opt)
	if err != nil {
		// 스트림을 못 만들었으면 여기서 종료 기록
		if obs != nil {
			call := observer.ProviderCall{
				CreatedAt: time.Now(),

				Provider: p.ProviderName,
				Model:    p.ModelName,
				Scenario: string(opt.Scenario),

				OS:                 opt.OS,
				Arch:               opt.Arch,
				MachineFingerprint: opt.MachineFingerprint,

				InputTokens:  0,
				OutputTokens: 0,
				LatencyMs:    time.Since(start).Milliseconds(),

				Success: false,
				Error:   provider.ShortError(err),
				Metadata: mergeMeta(opt.Metadata, map[string]any{
					"stream": true,
					"phase":  "open",
				}),
			}
			_ = obs.OnComplete(ctx, call)
		}
		return nil, err
	}

	// 스트림 종료를 포착하는 wrapper로 감싼다
	w := &streaming.WrapStream{
		Inner: st,

		Ctx: ctx,

		StartedAt: start,
		Provider:  p.ProviderName,
		Model:     p.ModelName,
		Scenario:  opt.Scenario,

		OS:                 opt.OS,
		Arch:               opt.Arch,
		MachineFingerprint: opt.MachineFingerprint,

		InputTokens:  0,
		OutputTokens: 0,

		Collector: &streaming.StreamCollector{},
		Observer:  obs,
	}
	return w, nil
}

func mergeMeta(a, b map[string]any) map[string]any {
	if a == nil && b == nil {
		return nil
	}
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}


---

7) Provider Registry에서 “무조건 Instrumentation 적용”

7.1 internal/provider/registry.go (MOD)

package provider

import (
	"errors"

	"devorch/internal/provider/instrument"
	"devorch/internal/provider/observer"
)

// ProviderFactory: 실제 구현체를 만드는 함수 타입
type ProviderFactory func() (Provider, string /*modelName*/, error)

// Registry: provider들을 등록/생성
type Registry struct {
	factories map[string]ProviderFactory
	observer  observer.Observer
}

func NewRegistry(obs observer.Observer) *Registry {
	return &Registry{
		factories: map[string]ProviderFactory{},
		observer:  obs,
	}
}

func (r *Registry) Register(name string, f ProviderFactory) {
	r.factories[name] = f
}

func (r *Registry) Create(name string) (Provider, error) {
	f, ok := r.factories[name]
	if !ok {
		return nil, errors.New("provider not registered: " + name)
	}

	p, modelName, err := f()
	if err != nil {
		return nil, err
	}

	// ✅ Step12 핵심: 모든 provider를 무조건 래핑
	wrapped := instrument.Wrap(p, name, modelName, r.observer)
	return wrapped, nil
}


---

8) Session/Tool에서 Scenario 주입(“어디서 호출됐는지”를 고정)

여기서부터가 “학습 품질”을 좌우하는 진짜 핵심입니다.
LLM 호출은 여러 군데에서 일어나는데, 결국 “Scenario를 정확히 넣어야” 라우터가 학습을 제대로 합니다.

8.1 internal/session/llm.go (NEW)

package session

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/instrument"
)

type LLMCaller struct {
	Provider provider.Provider

	OS                 string
	Arch               string
	MachineFingerprint string
}

func (c *LLMCaller) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	opt := provider.CallOptions{
		Scenario: instrument.ScenarioChat,

		OS:                 c.OS,
		Arch:               c.Arch,
		MachineFingerprint: c.MachineFingerprint,

		Observer: nil, // registry에 global observer가 있으면 nil이어도 기록됨
		Metadata: map[string]any{
			"layer": "session",
		},
	}
	return c.Provider.Chat(ctx, req, opt)
}

func (c *LLMCaller) ChatStream(ctx context.Context, req provider.ChatRequest) (provider.Stream[provider.ChatDelta], error) {
	opt := provider.CallOptions{
		Scenario: instrument.ScenarioChat,

		OS:                 c.OS,
		Arch:               c.Arch,
		MachineFingerprint: c.MachineFingerprint,

		Observer: nil,
		Metadata: map[string]any{
			"layer": "session",
		},
	}
	return c.Provider.ChatStream(ctx, req, opt)
}

8.2 internal/session/toolcall.go (NEW)

package session

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/instrument"
)

// ToolLLMCaller: tool 실행 중에 LLM을 호출하는 경우 scenario를 tool-call로 고정
type ToolLLMCaller struct {
	Provider provider.Provider

	OS                 string
	Arch               string
	MachineFingerprint string

	ToolName string
}

func (c *ToolLLMCaller) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	opt := provider.CallOptions{
		Scenario: instrument.ScenarioToolCall,

		OS:                 c.OS,
		Arch:               c.Arch,
		MachineFingerprint: c.MachineFingerprint,

		Metadata: map[string]any{
			"layer": "tool",
			"tool":  c.ToolName,
		},
	}
	return c.Provider.Chat(ctx, req, opt)
}


---

9) server/routes/provider.go에서 scenario 받기(옵션)

9.1 internal/server/routes/provider.go (MOD - 샘플)

package routes

import (
	"encoding/json"
	"net/http"
	"strings"

	"devorch/internal/provider"
	"devorch/internal/provider/instrument"
)

type ProviderRoutes struct {
	Provider provider.Provider

	OS                 string
	Arch               string
	MachineFingerprint string
}

type chatReq struct {
	Messages []provider.ChatMessage `json:"messages"`
	Scenario string                `json:"scenario,omitempty"`
	Stream   bool                  `json:"stream,omitempty"`
}

func (r *ProviderRoutes) HandleChat(w http.ResponseWriter, req *http.Request) {
	var body chatReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	sc := instrument.NormalizeScenario(strings.TrimSpace(body.Scenario))
	if sc == instrument.ScenarioOther {
		sc = instrument.ScenarioChat
	}

	opt := provider.CallOptions{
		Scenario: sc,

		OS:                 r.OS,
		Arch:               r.Arch,
		MachineFingerprint: r.MachineFingerprint,

		Metadata: map[string]any{
			"layer": "http-api",
		},
	}

	// non-stream 샘플
	resp, err := r.Provider.Chat(req.Context(), provider.ChatRequest{Messages: body.Messages}, opt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	_ = json.NewEncoder(w).Encode(resp)
}


---

10) Step12 “진짜로 동작하게” app wiring에서 observer 주입

Step11에서 만들었던 BenchRecorderObserver를 registry에 붙이면 끝입니다.

예시 (당신 app wiring 쪽에 추가):

// deps := app.BuildStep11(db)
benchObs := observer.NewBenchRecorderObserver(deps.Recorder)

// registry := provider.NewRegistry(benchObs)
// registry.Register("ollama", ...)  // Create() 시 자동으로 instrument.Wrap 적용됨


---

Step 12 결과: 무엇이 달라지나?

✅ 어떤 provider든(ollama/openai/anthropic/openrouter) 반드시 호출 종료 기록됨

✅ streaming도 EOF/에러/Close() 모두 기록됨

✅ session/tool/api 어디서 호출했는지 Scenario가 고정되어 라우팅 학습 품질이 올라감

✅ Step11의 /v1/router/choose 디버그와 Step10의 집계가 실사용 벤치로 누적됨



---

다음 Step 13에서 바로 하는 것(중요)

Step12까지는 “기록”이 완성입니다.
Step13 = 이 기록을 기반으로 라우터가 자동으로 “모델/프로바이더 우선순위”를 업데이트하고,

로컬(ollama) 성능이 좋은 OS/머신에서는 로컬 우선

약하면 OpenRouter/free pool 우선

실패율 높은 모델은 자동 회피 같은 “운영형 학습”까지 완성합니다.


원하면 지금 바로 Step 13 풀코드 이어서 작성할게요.