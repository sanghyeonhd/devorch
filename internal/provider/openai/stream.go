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

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("openai stream http " + res.Status)
	}

	type delta struct {
		Content string `json:"content"`
	}
	type choice struct {
		Delta        delta   `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	}
	type chunk struct {
		Choices []choice `json:"choices"`
	}

	var full bytes.Buffer
	var last provider.ChatResponse
	last.Provider_ = req.Provider
	last.Model_ = req.Model

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

	last.RawText = full.String()
	last.Text_ = full.String()
	// Usage는 stream에서 별도 endpoint/헤더로 받거나 추후 추정(6.5에서 개선)
	return last, nil
}

var _ = time.Second
