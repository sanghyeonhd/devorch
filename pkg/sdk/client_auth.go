package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

type Client struct {
	BaseURL   string
	Provider  string
	Workspace string
	HTTP      *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL:   baseURL,
		Provider:  "github",
		Workspace: "local",
		HTTP:      http.DefaultClient,
	}
}

func (c *Client) headers(req *http.Request) {
	if c.Provider != "" {
		req.Header.Set("X-DevOrch-Provider", c.Provider)
	}
	if c.Workspace != "" {
		req.Header.Set("X-DevOrch-Workspace", c.Workspace)
	}
}

func (c *Client) PostJSON(ctx context.Context, path string, body any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	c.headers(req)
	return c.HTTP.Do(req)
}
