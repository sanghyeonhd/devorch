좋아요. **6단계는 “멀티 LLM 실동작(스트리밍 포함) + 벤치마크/이력 저장(SQL 포함) + 라우터가 실측 결과로 점점 더 잘 고르는 구조”**를 컴파일 가능한 MVP 풀코드로 제공합니다.

> 전제: 1~5단계에서 internal/config/jsonc.go, internal/storage/sqlite/sqlite.go, internal/auth/*, internal/provider/registry.go 같은 기본 뼈대가 이미 존재한다고 가정합니다.
6단계는 새 파일 + 일부 수정 파일로 구성됩니다.




---

6단계 구현 목표

1. Provider “정식” 호출(최소):



OpenAI(OpenAI-compatible 포함 OpenRouter)

Anthropic

Ollama(로컬)

Google Gemini(REST는 Provider 별로 구조가 달라 MVP에서는 “비스트리밍 + 확장 포인트”)


2. SSE 스트리밍 파서(OpenAI/Anthropic/Ollama OpenAI-compat 시 공통)


3. 벤치마크(실측) 기록:



latency, token, error, provider/model, workspace, task_id/session_id

SQL 마이그레이션 포함


4. 라우터가 실측 점수 기반으로 점점 더 잘 선택



기존 router.score.go에 “벤치 기반 보정” 레이어 추가



---

6단계 파일 트리(추가/변경)

추가

internal/provider/types.go

internal/provider/httpx/sse.go

internal/provider/openai/chat.go

internal/provider/openai/stream.go

internal/provider/openrouter/chat.go

internal/provider/anthropic/chat.go

internal/provider/anthropic/stream.go

internal/provider/ollama/chat.go

internal/provider/ollama/stream.go

internal/provider/google/chat.go  (MVP: non-stream)

internal/bench/bench.go

internal/bench/recorder.go

internal/storage/sqlite/bench.go

internal/storage/sqlite/migrations/0003_bench.sql


수정(핵심)

internal/provider/registry.go (provider별 Chat 호출 연결)

internal/session/processor.go (응답 종료 시 bench 기록)

internal/router/score.go (bench 기반 보정)



---

6-1) Provider 공통 타입

internal/provider/types.go

package provider

import (
	"context"
	"time"
)

type Role string

const (
	RoleSystem Role = "system"
	RoleUser   Role = "user"
	RoleAssist Role = "assistant"
	RoleTool   Role = "tool"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type ChatRequest struct {
	Provider   string
	Model      string
	Messages   []Message
	Temperature float64
	MaxTokens  int
	Stream     bool

	// routing/meta
	Workspace string
	SessionID string
	TaskID    string
}

type ChatResponse struct {
	Text      string
	Model     string
	Provider  string
	Usage     Usage
	ToolCalls []ToolCall
	RawJSON   []byte
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	TextDelta string
	Done      bool
	// tool deltas etc (확장 포인트)
}

type ChatClient interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, yield func(StreamChunk) error) (ChatResponse, error)
}

type BenchResult struct {
	Workspace  string
	SessionID  string
	TaskID     string
	Provider   string
	Model      string
	StartedAt  time.Time
	EndedAt    time.Time
	LatencyMs  int64
	PromptTok  int
	CompletionTok int
	TotalTok   int
	Error      string
}


---

6-2) SSE 파서(공통)

internal/provider/httpx/sse.go

package httpx

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
)

type SSEEvent struct {
	Data string
}

func ReadSSE(ctx context.Context, r io.Reader, onEvent func(SSEEvent) error) error {
	sc := bufio.NewScanner(r)
	// SSE payload가 길면 기본 토큰 제한에 걸림 → 버퍼 확장
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 8*1024*1024)

	var data bytes.Buffer

	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		ev := SSEEvent{Data: strings.TrimSpace(data.String())}
		data.Reset()
		return onEvent(ev)
	}

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := sc.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			data.WriteString(payload)
			data.WriteString("\n")
		}
		// id:, event: 무시 (확장 가능)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return flush()
}


---

6-3) OpenAI Provider (스트리밍 포함)

internal/provider/openai/chat.go

package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"devorch/internal/provider"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	Auth    func(req *http.Request) error // Authorization 주입(5단계 AuthInjector 사용)
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 90 * time.Second}
	}
	u := c.BaseURL
	if u == "" {
		u = "https://api.openai.com"
	}
	endpoint := u + "/v1/chat/completions"

	payload := map[string]any{
		"model":       req.Model,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]any{
			"role":    string(m.Role),
			"content": m.Content,
			"name":    m.Name,
		})
	}
	payload["messages"] = msgs

	b, _ := json.Marshal(payload)
	hreq, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	if c.Auth != nil {
		if err := c.Auth(hreq); err != nil {
			return provider.ChatResponse{}, err
		}
	}

	res, err := c.HTTP.Do(hreq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("openai http " + res.Status + ": " + string(raw))
	}

	return parseOpenAIResponse(raw, req.Provider, req.Model)
}

func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	return stream(c, ctx, req, yield)
}

func parseOpenAIResponse(raw []byte, prov string, model string) (provider.ChatResponse, error) {
	type choiceMsg struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	type resp struct {
		Model  string `json:"model"`
		Choices []choiceMsg `json:"choices"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	var r resp
	if err := json.Unmarshal(raw, &r); err != nil {
		return provider.ChatResponse{}, err
	}
	text := ""
	if len(r.Choices) > 0 {
		text = r.Choices[0].Message.Content
	}
	return provider.ChatResponse{
		Text:     text,
		Model:    firstNonEmpty(r.Model, model),
		Provider: prov,
		Usage: provider.Usage{
			PromptTokens: r.Usage.PromptTokens,
			CompletionTokens: r.Usage.CompletionTokens,
			TotalTokens: r.Usage.TotalTokens,
		},
		RawJSON: raw,
	}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" { return a }
	return b
}

internal/provider/openai/stream.go

package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"devorch/internal/provider"
	"devorch/internal/provider/httpx"
)

func stream(c *Client, ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 0} // streaming: no timeout (or long)
	}
	u := c.BaseURL
	if u == "" {
		u = "https://api.openai.com"
	}
	endpoint := u + "/v1/chat/completions"

	payload := map[string]any{
		"model":       req.Model,
		"temperature": req.Temperature,
		"stream":      true,
	}
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]any{
			"role":    string(m.Role),
			"content": m.Content,
			"name":    m.Name,
		})
	}
	payload["messages"] = msgs
	b, _ := json.Marshal(payload)

	hreq, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	if c.Auth != nil {
		if err := c.Auth(hreq); err != nil {
			return provider.ChatResponse{}, err
		}
	}

	res, err := c.HTTP.Do(hreq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("openai stream http " + res.Status)
	}

	type delta struct {
		Content string `json:"content"`
	}
	type choice struct {
		Delta        delta  `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	}
	type chunk struct {
		Choices []choice `json:"choices"`
	}

	var full bytes.Buffer
	var last provider.ChatResponse
	last.Provider = req.Provider
	last.Model = req.Model

	err = httpx.ReadSSE(ctx, res.Body, func(ev httpx.SSEEvent) error {
		if ev.Data == "[DONE]" {
			if err := yield(provider.StreamChunk{Done: true}); err != nil {
				return err
			}
			return nil
		}
		var ch chunk
		if err := json.Unmarshal([]byte(ev.Data), &ch); err != nil {
			// 일부 이벤트가 빈줄/잡데이터일 수 있어 무시하지 않고 에러 처리
			return err
		}
		if len(ch.Choices) > 0 {
			d := ch.Choices[0].Delta.Content
			if d != "" {
				full.WriteString(d)
				if err := yield(provider.StreamChunk{TextDelta: d}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return provider.ChatResponse{}, err
	}

	last.Text = full.String()
	// Usage는 stream에서 별도 endpoint/헤더로 받거나 추후 추정(6.5에서 개선)
	return last, nil
}

var _ = time.Second


---

6-4) OpenRouter Provider (OpenAI 호환)

internal/provider/openrouter/chat.go

package openrouter

import (
	"devorch/internal/provider"
	"devorch/internal/provider/openai"
)

type Client struct {
	OpenAI *openai.Client
}

func New(auth func(req *http.Request) error) *Client {
	// NOTE: openrouter는 base url이 다름
	return &Client{
		OpenAI: &openai.Client{
			BaseURL: "https://openrouter.ai/api",
			Auth:    auth,
		},
	}
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return c.OpenAI.Chat(ctx, req)
}

func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	return c.OpenAI.ChatStream(ctx, req, yield)
}

> 위 파일은 net/http import가 필요합니다. 컴파일을 위해 아래처럼 수정하세요:



import (
  "context"
  "net/http"
  "devorch/internal/provider"
  "devorch/internal/provider/openai"
)


---

6-5) Anthropic Provider (MVP 스트리밍)

Anthropic의 메시지 포맷은 OpenAI와 다르기 때문에 별도 구현합니다.

internal/provider/anthropic/chat.go

package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"devorch/internal/provider"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	Auth    func(req *http.Request) error // "x-api-key" 혹은 oauth bearer 등. (AuthInjector가 bearer면 헤더명 변환이 필요할 수 있음)
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 90 * time.Second}
	}
	u := c.BaseURL
	if u == "" {
		u = "https://api.anthropic.com"
	}
	endpoint := u + "/v1/messages"

	sys := ""
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == provider.RoleSystem {
			sys += m.Content + "\n"
			continue
		}
		role := "user"
		if m.Role == provider.RoleAssist {
			role = "assistant"
		}
		msgs = append(msgs, map[string]any{
			"role": role,
			"content": []map[string]any{
				{"type": "text", "text": m.Content},
			},
		})
	}

	payload := map[string]any{
		"model":    req.Model,
		"messages": msgs,
	}
	if sys != "" {
		payload["system"] = sys
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	} else {
		payload["max_tokens"] = 1024
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}

	b, _ := json.Marshal(payload)
	hreq, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	// Anthropic는 버전 헤더 요구
	hreq.Header.Set("anthropic-version", "2023-06-01")

	if c.Auth != nil {
		if err := c.Auth(hreq); err != nil {
			return provider.ChatResponse{}, err
		}
		// AuthInjector가 Authorization: Bearer 를 넣는다면, 실제 Anthropic은 x-api-key가 일반적.
		// OAuth-only를 강제한다면, enterprise gateway/oidc를 통해 Bearer로 받는 쪽으로 설계해야 함.
	}

	res, err := c.HTTP.Do(hreq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("anthropic http " + res.Status + ": " + string(raw))
	}

	type out struct {
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	var o out
	if err := json.Unmarshal(raw, &o); err != nil {
		return provider.ChatResponse{}, err
	}
	txt := ""
	for _, c := range o.Content {
		if c.Type == "text" {
			txt += c.Text
		}
	}

	return provider.ChatResponse{
		Text:     txt,
		Model:    firstNonEmpty(o.Model, req.Model),
		Provider: req.Provider,
		Usage: provider.Usage{
			PromptTokens:     o.Usage.InputTokens,
			CompletionTokens: o.Usage.OutputTokens,
			TotalTokens:      o.Usage.InputTokens + o.Usage.OutputTokens,
		},
		RawJSON: raw,
	}, nil
}

func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	return stream(c, ctx, req, yield)
}

func firstNonEmpty(a, b string) string { if a != "" { return a }; return b }

internal/provider/anthropic/stream.go

package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"devorch/internal/provider"
	"devorch/internal/provider/httpx"
)

func stream(c *Client, ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 0}
	}
	u := c.BaseURL
	if u == "" {
		u = "https://api.anthropic.com"
	}
	endpoint := u + "/v1/messages"

	sys := ""
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == provider.RoleSystem {
			sys += m.Content + "\n"
			continue
		}
		role := "user"
		if m.Role == provider.RoleAssist {
			role = "assistant"
		}
		msgs = append(msgs, map[string]any{
			"role": role,
			"content": []map[string]any{
				{"type": "text", "text": m.Content},
			},
		})
	}

	payload := map[string]any{
		"model":    req.Model,
		"messages": msgs,
		"stream":   true,
		"max_tokens": 1024,
	}
	if sys != "" {
		payload["system"] = sys
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}

	b, _ := json.Marshal(payload)
	hreq, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("anthropic-version", "2023-06-01")
	if c.Auth != nil {
		if err := c.Auth(hreq); err != nil {
			return provider.ChatResponse{}, err
		}
	}

	res, err := c.HTTP.Do(hreq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("anthropic stream http " + res.Status)
	}

	// Anthropic stream: event data JSON (type: content_block_delta 등)
	var full bytes.Buffer
	last := provider.ChatResponse{Provider: req.Provider, Model: req.Model}

	err = httpx.ReadSSE(ctx, res.Body, func(ev httpx.SSEEvent) error {
		var v map[string]any
		if err := json.Unmarshal([]byte(ev.Data), &v); err != nil {
			return err
		}
		typ, _ := v["type"].(string)
		if typ == "content_block_delta" {
			d, _ := v["delta"].(map[string]any)
			if d != nil {
				if txt, _ := d["text"].(string); txt != "" {
					full.WriteString(txt)
					return yield(provider.StreamChunk{TextDelta: txt})
				}
			}
		}
		if typ == "message_stop" {
			return yield(provider.StreamChunk{Done: true})
		}
		return nil
	})
	if err != nil {
		return provider.ChatResponse{}, err
	}

	last.Text = full.String()
	return last, nil
}

var _ = time.Second


---

6-6) Ollama Provider (로컬, OpenAI 호환 API 기준)

Ollama는 /api/chat도 있고 OpenAI-compat endpoint도 있습니다.
MVP는 Ollama OpenAI-compat(http://127.0.0.1:11434/v1/chat/completions) 기준으로 통일하면 쉽습니다.

internal/provider/ollama/chat.go

package ollama

import (
	"context"
	"net/http"

	"devorch/internal/provider"
	"devorch/internal/provider/openai"
)

type Client struct {
	OpenAI *openai.Client
}

func NewLocal() *Client {
	return &Client{
		OpenAI: &openai.Client{
			BaseURL: "http://127.0.0.1:11434",
			HTTP:    &http.Client{},
			Auth:    nil, // 로컬은 보통 auth 없음
		},
	}
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return c.OpenAI.Chat(ctx, req)
}

func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	return c.OpenAI.ChatStream(ctx, req, yield)
}


---

6-7) Google Gemini Provider (MVP: non-stream + 확장 포인트)

> Gemini 스트리밍/툴콜은 API 형태가 OpenAI/Anthropic과 달라 MVP에서는 “non-stream”로 두고, 6.5에서 스트림/툴콜 확장하는 게 안전합니다.



internal/provider/google/chat.go

package google

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"devorch/internal/provider"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	Auth    func(req *http.Request) error
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 90 * time.Second}
	}
	// 실제 Gemini endpoint는 모델/버전마다 다름 → 여기서는 “gateway/compat”를 전제로 둠
	if c.BaseURL == "" {
		return provider.ChatResponse{}, errors.New("google provider requires gateway base_url")
	}
	endpoint := c.BaseURL + "/chat"

	payload := map[string]any{
		"model": req.Model,
		"messages": req.Messages,
		"temperature": req.Temperature,
		"max_tokens": req.MaxTokens,
	}
	b, _ := json.Marshal(payload)
	hreq, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	if c.Auth != nil {
		if err := c.Auth(hreq); err != nil {
			return provider.ChatResponse{}, err
		}
	}

	res, err := c.HTTP.Do(hreq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("google http " + res.Status + ": " + string(raw))
	}
	// gateway가 OpenAI-like로 내려준다고 가정
	return provider.ChatResponse{Text: string(raw), Provider: req.Provider, Model: req.Model, RawJSON: raw}, nil
}

func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	// MVP: 비지원
	return provider.ChatResponse{}, errors.New("google stream not implemented in MVP")
}


---

6-8) Provider Registry 연결 (핵심)

internal/provider/registry.go (수정/완성본 예시)

package provider

import (
	"context"
	"net/http"

	"devorch/internal/provider/anthropic"
	"devorch/internal/provider/openai"
	"devorch/internal/provider/ollama"
)

type Registry struct {
	clients map[string]ChatClient
}

func NewRegistry(injector *AuthInjector) *Registry {
	authFn := func(pname string) func(*http.Request) error {
		return func(r *http.Request) error {
			return injector.Inject(context.Background(), r, pname) // NOTE: 6.2에서 ctx 전달형으로 개선
		}
	}

	r := &Registry{clients: map[string]ChatClient{}}

	// OpenAI
	r.clients["openai"] = &openai.Client{
		BaseURL: "https://api.openai.com",
		Auth:    authFn("openai"),
	}

	// OpenRouter(OpenAI compat)
	r.clients["openrouter"] = &openai.Client{
		BaseURL: "https://openrouter.ai/api",
		Auth:    authFn("openrouter"),
	}

	// Anthropic
	r.clients["anthropic"] = &anthropic.Client{
		BaseURL: "https://api.anthropic.com",
		Auth:    authFn("anthropic"),
	}

	// Local Ollama
	r.clients["ollama"] = ollama.NewLocal()

	return r
}

func (r *Registry) Get(providerName string) (ChatClient, bool) {
	c, ok := r.clients[providerName]
	return c, ok
}

func (r *Registry) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	c, ok := r.Get(req.Provider)
	if !ok {
		return ChatResponse{}, ErrProviderNotFound
	}
	if req.Stream {
		return c.ChatStream(ctx, req, func(_ StreamChunk) error { return nil })
	}
	return c.Chat(ctx, req)
}

var ErrProviderNotFound = &ProviderError{Kind: "provider_not_found"}

type ProviderError struct{ Kind string }
func (e *ProviderError) Error() string { return e.Kind }

> ⚠️ 여기서 injector.Inject(context.Background(), ...)로 넣은 이유는 5단계 Manager가 workspace를 ctx에서 못 읽는 구조였기 때문입니다.
6.2에서 AuthInjector.Inject(ctx, req, provider)로 ctx 전달을 유지하도록 바로 정리합니다.




---

6-9) Bench(실측) 기록: 테이블 + 저장 코드

internal/storage/sqlite/migrations/0003_bench.sql

CREATE TABLE IF NOT EXISTS bench_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  session_id TEXT,
  task_id TEXT,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  latency_ms INTEGER NOT NULL,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  error TEXT
);

CREATE INDEX IF NOT EXISTS idx_bench_runs_ws_time ON bench_runs(workspace, started_at);
CREATE INDEX IF NOT EXISTS idx_bench_runs_provider_model ON bench_runs(provider, model, started_at);

internal/storage/sqlite/bench.go

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/id"
	"devorch/internal/provider"
)

type BenchStore struct {
	DB *sql.DB
}

func (s *BenchStore) Insert(ctx context.Context, br provider.BenchResult) (string, error) {
	if br.StartedAt.IsZero() {
		br.StartedAt = time.Now()
	}
	if br.EndedAt.IsZero() {
		br.EndedAt = time.Now()
	}
	if br.LatencyMs == 0 {
		br.LatencyMs = br.EndedAt.Sub(br.StartedAt).Milliseconds()
	}
	rid := id.NewULID()

	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO bench_runs (
		  id, workspace, session_id, task_id, provider, model,
		  started_at, ended_at, latency_ms,
		  prompt_tokens, completion_tokens, total_tokens, error
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, rid, br.Workspace, br.SessionID, br.TaskID, br.Provider, br.Model,
		br.StartedAt.UnixMilli(), br.EndedAt.UnixMilli(), br.LatencyMs,
		br.PromptTok, br.CompletionTok, br.TotalTok, br.Error,
	)
	return rid, err
}

type BenchAgg struct {
	Provider string
	Model    string
	Count    int
	P50LatencyMs int64
	ErrorRate float64
}

func (s *BenchStore) RecentAgg(ctx context.Context, workspace string, limit int) ([]BenchAgg, error) {
	// MVP: 아주 단순 집계(정교한 p50은 7단계에서)
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT provider, model,
	         COUNT(*) as cnt,
	         AVG(latency_ms) as avg_latency,
	         AVG(CASE WHEN error IS NULL OR error='' THEN 0 ELSE 1 END) as err_rate
	  FROM bench_runs
	  WHERE workspace=?
	  GROUP BY provider, model
	  ORDER BY cnt DESC
	  LIMIT ?
	`, workspace, limit)
	if err != nil { return nil, err }
	defer rows.Close()

	out := []BenchAgg{}
	for rows.Next() {
		var a BenchAgg
		var avg float64
		if err := rows.Scan(&a.Provider, &a.Model, &a.Count, &avg, &a.ErrorRate); err != nil {
			return nil, err
		}
		a.P50LatencyMs = int64(avg)
		out = append(out, a)
	}
	return out, nil
}

> id.NewULID() 는 3~4단계에 이미 만든 internal/id/ulid.go를 사용합니다.




---

6-10) Bench Recorder (Provider 호출을 감싸서 자동 기록)

internal/bench/bench.go

package bench

import (
	"context"
	"time"

	"devorch/internal/provider"
)

type Store interface {
	Insert(ctx context.Context, br provider.BenchResult) (string, error)
}

type Recorder struct {
	Store Store
}

func (r *Recorder) WrapChat(ctx context.Context, req provider.ChatRequest, fn func() (provider.ChatResponse, error)) (provider.ChatResponse, error) {
	start := time.Now()
	resp, err := fn()
	end := time.Now()

	br := provider.BenchResult{
		Workspace: req.Workspace,
		SessionID: req.SessionID,
		TaskID:    req.TaskID,
		Provider:  req.Provider,
		Model:     req.Model,
		StartedAt: start,
		EndedAt:   end,
		LatencyMs: end.Sub(start).Milliseconds(),
	}
	if err != nil {
		br.Error = err.Error()
	} else {
		br.PromptTok = resp.Usage.PromptTokens
		br.CompletionTok = resp.Usage.CompletionTokens
		br.TotalTok = resp.Usage.TotalTokens
	}
	if r.Store != nil {
		_, _ = r.Store.Insert(ctx, br) // bench 저장 실패는 본 요청 실패로 만들지 않음
	}
	return resp, err
}


---

6-11) Session Processor에서 provider 호출을 “Bench Recorder로 감싸기”

internal/session/processor.go (핵심 부분만 “완성본 예시”)

package session

import (
	"context"

	"devorch/internal/bench"
	"devorch/internal/provider"
)

type Processor struct {
	Providers *provider.Registry
	Bench     *bench.Recorder
	// Router/Scheduler etc...
}

func (p *Processor) RunChat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	call := func() (provider.ChatResponse, error) {
		c, ok := p.Providers.Get(req.Provider)
		if !ok {
			return provider.ChatResponse{}, provider.ErrProviderNotFound
		}
		if req.Stream {
			return c.ChatStream(ctx, req, func(ch provider.StreamChunk) error {
				// TODO: bus/SSE로 UI에 흘려보내기 (7단계에서 session.stream.go와 연결)
				return nil
			})
		}
		return c.Chat(ctx, req)
	}

	if p.Bench != nil {
		return p.Bench.WrapChat(ctx, req, call)
	}
	return call()
}


---

6-12) Router Score에 “Bench 기반 보정” 추가

internal/router/score.go (추가 함수 + 기존 score에 가중치 반영)

package router

import (
	"context"
	"math"

	"devorch/internal/storage/sqlite"
)

type BenchReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.BenchAgg, error)
}

// Score: 기존 latency/cost/quality 추정치 + 실측 보정
type Scorer struct {
	Bench BenchReader
}

func (s *Scorer) BenchPenalty(ctx context.Context, workspace, providerName, model string) float64 {
	if s.Bench == nil {
		return 0
	}
	aggs, err := s.Bench.RecentAgg(ctx, workspace, 50)
	if err != nil {
		return 0
	}
	for _, a := range aggs {
		if a.Provider == providerName && a.Model == model {
			lat := float64(a.P50LatencyMs)
			errRate := a.ErrorRate
			// penalty는 낮을수록 좋음. 지수 형태로 완만하게.
			p := math.Log1p(lat/500.0) + (errRate * 2.0)
			return p
		}
	}
	return 0
}

> 실제 라우팅 함수(router.router.go)에서 후보 모델 점수 계산할 때
score += BenchPenalty(...) 형태로 합치면 됩니다.




---

6-13) 앱 와이어링(bench store 주입)

예: internal/app/app.go에서

sqlite db 준비 후

BenchStore → bench.Recorder → session.Processor에 주입


(프로젝트마다 구조가 달라서 최소 예시만)

bs := &sqlite.BenchStore{DB: db}
rec := &bench.Recorder{Store: bs}
processor := &session.Processor{
  Providers: providerRegistry,
  Bench: rec,
}


---

6단계 “완료” 체크리스트

1. OpenAI(또는 OpenRouter)로 non-stream 요청 1회
→ bench_runs에 1 row 저장 확인


2. OpenAI stream 요청 1회
→ 스트리밍 delta는 yield 경로로 들어오고, 종료 시 bench row 저장


3. Ollama 로컬 stream 요청 1회
→ bench row 저장


4. Router score에 BenchPenalty 합산(최소)
→ 동일 workspace에서 “느린 모델/에러 많은 모델” 점수가 자동으로 불리해짐




---

다음 단계(7단계)에서 반드시 붙일 것(권장)

스트리밍을 bus(SSE/ws)로 실제 UI까지 전달(지금은 TODO로 남겨둔 부분)

벤치 집계의 p50/p95 계산(현재는 AVG 기반)

“작업 완료 기준” 평가(예: 테스트 통과, lint 통과, diff 품질) → quality score 저장

벤치/품질/비용을 합친 학습형 라우팅 정책(이력이 쌓일수록 더 정확)



---

원하면, 바로 이어서 **7단계 풀코드(Streaming → Bus → VSCode/WebUI 실시간 렌더 + 품질평가/자동학습 라우팅)**로 계속 진행할게요.