package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devorch/internal/provider"
)

type tagsResp struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func Tags(ctx context.Context, host string) ([]string, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, provider.Err("ollama_tags_failed", "non-2xx /api/tags")
	}
	var tr tagsResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(tr.Models))
	for _, m := range tr.Models {
		out = append(out, m.Name)
	}
	return out, nil
}

type chatReq struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

type chatResp struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	PromptEvalCount    int `json:"prompt_eval_count"`
	EvalCount          int `json:"eval_count"`
	TotalDuration      int `json:"total_duration"`
	PromptEvalDuration int `json:"prompt_eval_duration"`
	EvalDuration       int `json:"eval_duration"`
}

func Chat(ctx context.Context, host string, req provider.ChatRequest) (string, int, int, int, error) {
	c := &http.Client{Timeout: 60 * time.Second}

	payload := chatReq{Model: req.Model, Stream: false}
	payload.Messages = make([]struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}, 0, len(req.Messages))
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: m.Role, Content: m.Content})
	}

	b, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return "", 0, 0, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(httpReq)
	if err != nil {
		return "", 0, 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", 0, 0, 0, provider.Err("ollama_chat_failed", "non-2xx /api/chat")
	}

	var cr chatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", 0, 0, 0, err
	}

	pt := cr.PromptEvalCount
	ct := cr.EvalCount
	tt := pt + ct
	return cr.Message.Content, pt, ct, tt, nil
}
