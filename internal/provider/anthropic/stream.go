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
		"model":      req.Model,
		"messages":   msgs,
		"stream":     true,
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

	var full bytes.Buffer
	last := provider.ChatResponse{Provider_: req.Provider, Model_: req.Model}

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

	last.RawText = full.String()
	last.Text_ = full.String()
	return last, nil
}

var _ = time.Second
