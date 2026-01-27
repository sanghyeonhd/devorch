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
			"role":    m.Role,
			"content": m.Content,
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
		Model   string      `json:"model"`
		Choices []choiceMsg `json:"choices"`
		Usage   struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
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
		RawText:          text,
		Text_:            text,
		Model_:           firstNonEmpty(r.Model, model),
		Provider_:        prov,
		PromptTokens:     r.Usage.PromptTokens,
		CompletionTokens: r.Usage.CompletionTokens,
		TotalTokens:      r.Usage.TotalTokens,
		Usage_: provider.Usage{
			PromptTokens:     r.Usage.PromptTokens,
			CompletionTokens: r.Usage.CompletionTokens,
			TotalTokens:      r.Usage.TotalTokens,
		},
		RawJSON: raw,
	}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
