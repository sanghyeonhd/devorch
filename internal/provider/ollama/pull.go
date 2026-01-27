package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devorch/internal/provider"
)

type pullReq struct {
	Name string `json:"name"`
}

func Pull(ctx context.Context, host, model string) error {
	c := &http.Client{Timeout: 0} // pull can take long
	b, _ := json.Marshal(pullReq{Name: model})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/pull", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return provider.Err("ollama_pull_failed", "non-2xx /api/pull")
	}
	// response is a stream of JSON lines; Step2: treat 2xx as accepted
	_ = time.Second // keep import stable if needed
	return nil
}
