package google

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
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
	// 실제 Gemini endpoint는 모델/버전마다 다름 → 여기서는 "gateway/compat"를 전제로 둠
	if c.BaseURL == "" {
		return provider.ChatResponse{}, errors.New("google provider requires gateway base_url")
	}
	endpoint := c.BaseURL + "/chat"

	payload := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
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
	return provider.ChatResponse{
		RawText:   string(raw),
		Text_:     string(raw),
		Provider_: req.Provider,
		Model_:    req.Model,
		RawJSON:   raw,
	}, nil
}

func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(provider.StreamChunk) error) (provider.ChatResponse, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 120 * time.Second}
	}
	if c.BaseURL == "" {
		return provider.ChatResponse{}, errors.New("google provider requires gateway base_url")
	}
	endpoint := c.BaseURL + "/chat"

	payload := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
	}
	b, _ := json.Marshal(payload)
	hreq, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "text/event-stream")
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
		raw, _ := io.ReadAll(res.Body)
		return provider.ChatResponse{}, errors.New("google http " + res.Status + ": " + string(raw))
	}

	var fullText strings.Builder
	scanner := bufio.NewScanner(res.Body)

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: data: {...}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		// Parse SSE chunk (OpenAI-compatible format)
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			text := chunk.Choices[0].Delta.Content
			fullText.WriteString(text)

			if err := yield(provider.StreamChunk{
				TextDelta: text,
				Done:      false,
			}); err != nil {
				return provider.ChatResponse{}, err
			}
		}
	}

	// Final response
	return provider.ChatResponse{
		RawText:   fullText.String(),
		Text_:     fullText.String(),
		Provider_: req.Provider,
		Model_:    req.Model,
	}, nil
}
