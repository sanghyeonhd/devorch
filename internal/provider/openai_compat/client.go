package openai_compat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"devorch/internal/provider"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	Headers map[string]string // optional extra headers
}

func New(baseURL, apiKey string, headers map[string]string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
		Headers: headers,
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var buf *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		// best-effort read
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		return provider.Err("http_error", fmt.Sprintf("status=%d body=%s", resp.StatusCode, b.String()))
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (c *Client) ListModels(ctx context.Context) ([]provider.Model, error) {
	var mr modelsResponse
	if err := c.do(ctx, http.MethodGet, "/models", nil, &mr); err != nil {
		return nil, err
	}
	out := make([]provider.Model, 0, len(mr.Data))
	for _, d := range mr.Data {
		out = append(out, provider.Model{ID: d.ID})
	}
	return out, nil
}

// Chat Completions compatible (widely supported)
type chatReq struct {
	Model       string                 `json:"model"`
	Messages    []provider.ChatMessage `json:"messages"`
	Temperature float64                `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	var cr chatResp
	payload := chatReq{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	if err := c.do(ctx, http.MethodPost, "/chat/completions", payload, &cr); err != nil {
		return provider.ChatResponse{}, err
	}
	txt := ""
	if len(cr.Choices) > 0 {
		txt = cr.Choices[0].Message.Content
	}
	return provider.ChatResponse{
		RawText:          txt,
		PromptTokens:     cr.Usage.PromptTokens,
		CompletionTokens: cr.Usage.CompletionTokens,
		TotalTokens:      cr.Usage.TotalTokens,
		// Step1: cost unknown unless pricing table -> 0
		CostMicroUSD: 0,
	}, nil
}

// ChatWithQueryParam sends a chat completion request with an additional query parameter
// Used for Azure OpenAI which requires api-version query param
func (c *Client) ChatWithQueryParam(ctx context.Context, req provider.ChatRequest, key, value string) (provider.ChatResponse, error) {
	var cr chatResp
	payload := chatReq{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	path := fmt.Sprintf("/chat/completions?%s=%s", key, value)
	if err := c.do(ctx, http.MethodPost, path, payload, &cr); err != nil {
		return provider.ChatResponse{}, err
	}
	txt := ""
	if len(cr.Choices) > 0 {
		txt = cr.Choices[0].Message.Content
	}
	return provider.ChatResponse{
		RawText:          txt,
		PromptTokens:     cr.Usage.PromptTokens,
		CompletionTokens: cr.Usage.CompletionTokens,
		TotalTokens:      cr.Usage.TotalTokens,
		CostMicroUSD:     0,
	}, nil
}

func (c *Client) Health(ctx context.Context) error {
	// simplest: list models with short timeout
	_, err := c.ListModels(ctx)
	return err
}
