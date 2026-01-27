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
	Auth    func(req *http.Request) error
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
		if m.Role == "system" {
			sys += m.Content + "\n"
			continue
		}
		role := "user"
		if m.Role == "assistant" {
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
	for _, cnt := range o.Content {
		if cnt.Type == "text" {
			txt += cnt.Text
		}
	}

	return provider.ChatResponse{
		RawText:          txt,
		Text_:            txt,
		Model_:           firstNonEmpty(o.Model, req.Model),
		Provider_:        req.Provider,
		PromptTokens:     o.Usage.InputTokens,
		CompletionTokens: o.Usage.OutputTokens,
		TotalTokens:      o.Usage.InputTokens + o.Usage.OutputTokens,
		Usage_: provider.Usage{
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

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
