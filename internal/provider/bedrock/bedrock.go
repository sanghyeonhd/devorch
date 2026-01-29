package bedrock

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"devorch/internal/provider"
)

// Provider implements the AWS Bedrock provider
type Provider struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	region          string
	client          *http.Client
}

// New creates a new AWS Bedrock provider
// Reads credentials from AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN, AWS_REGION
func New() *Provider {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	return &Provider{
		accessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		secretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		sessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
		region:          region,
		client:          &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *Provider) Name() string { return "bedrock" }
func (p *Provider) Kind() string { return "remote" }
func (p *Provider) Endpoint() string {
	return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", p.region)
}

func (p *Provider) Health(ctx context.Context) error {
	if p.accessKeyID == "" || p.secretAccessKey == "" {
		return errors.New("AWS credentials not configured (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)")
	}
	return nil
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "anthropic.claude-3-5-sonnet-20240620-v1:0"},
		{ID: "anthropic.claude-3-opus-20240229-v1:0"},
		{ID: "anthropic.claude-3-sonnet-20240229-v1:0"},
		{ID: "anthropic.claude-3-haiku-20240307-v1:0"},
		{ID: "amazon.titan-text-express-v1"},
		{ID: "amazon.titan-text-lite-v1"},
		{ID: "meta.llama3-70b-instruct-v1:0"},
		{ID: "meta.llama3-8b-instruct-v1:0"},
		{ID: "mistral.mistral-7b-instruct-v0:2"},
		{ID: "mistral.mixtral-8x7b-instruct-v0:1"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if p.accessKeyID == "" || p.secretAccessKey == "" {
		return provider.ChatResponse{}, errors.New("AWS credentials not configured")
	}

	// Build prompt from messages
	var prompt strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			prompt.WriteString(fmt.Sprintf("\n\nHuman: %s\n\nAssistant: I understand.", msg.Content))
		case "user":
			prompt.WriteString(fmt.Sprintf("\n\nHuman: %s", msg.Content))
		case "assistant":
			prompt.WriteString(fmt.Sprintf("\n\nAssistant: %s", msg.Content))
		}
	}
	prompt.WriteString("\n\nAssistant:")

	// Bedrock Claude request format
	body := map[string]any{
		"prompt":               prompt.String(),
		"max_tokens_to_sample": req.MaxTokens,
		"temperature":          req.Temperature,
	}
	if req.MaxTokens == 0 {
		body["max_tokens_to_sample"] = 4096
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return provider.ChatResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build request URL
	modelID := req.Model
	if modelID == "" {
		modelID = "anthropic.claude-3-haiku-20240307-v1:0"
	}
	url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke", p.region, modelID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return provider.ChatResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Sign request with AWS Signature V4
	httpReq.Header.Set("Content-Type", "application/json")
	if err := p.signRequest(httpReq, bodyBytes); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return provider.ChatResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.ChatResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return provider.ChatResponse{}, fmt.Errorf("bedrock error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result struct {
		Completion string `json:"completion"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return provider.ChatResponse{
		Text_:     strings.TrimSpace(result.Completion),
		RawText:   result.Completion,
		Provider_: "bedrock",
		Model_:    modelID,
		RawJSON:   respBody,
	}, nil
}

// signRequest signs an HTTP request with AWS Signature Version 4
func (p *Provider) signRequest(req *http.Request, payload []byte) error {
	service := "bedrock"
	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	// Hash payload
	payloadHash := sha256Hash(payload)

	// Canonical headers
	req.Header.Set("host", req.Host)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	if p.sessionToken != "" {
		req.Header.Set("x-amz-security-token", p.sessionToken)
	}

	// Build canonical request
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	if p.sessionToken != "" {
		signedHeaders = "content-type;host;x-amz-content-sha256;x-amz-date;x-amz-security-token"
	}

	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"),
		req.Host,
		payloadHash,
		amzDate,
	)
	if p.sessionToken != "" {
		canonicalHeaders = fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\nx-amz-security-token:%s\n",
			req.Header.Get("Content-Type"),
			req.Host,
			payloadHash,
			amzDate,
			p.sessionToken,
		)
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	// Build string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, p.region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hash([]byte(canonicalRequest)),
	)

	// Calculate signature
	signingKey := getSignatureKey(p.secretAccessKey, dateStamp, p.region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Build authorization header
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		p.accessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authHeader)

	return nil
}

func sha256Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
