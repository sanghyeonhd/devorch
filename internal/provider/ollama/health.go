package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devorch/internal/provider"
)

func Health(ctx context.Context, host string) error {
	c := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/version", nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return provider.Err("ollama_unhealthy", "non-2xx /api/version")
	}
	return nil
}

type versionResp struct {
	Version string `json:"version"`
}

func Version(ctx context.Context, host string) (string, error) {
	c := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/version", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var vr versionResp
	_ = json.NewDecoder(resp.Body).Decode(&vr)
	return vr.Version, nil
}
