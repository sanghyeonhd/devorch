// Package tui provides terminal user interface components.
package tui

import (
	"bufio"
	"bytes"
	"context"
	"devorch/internal/auth"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ConnectProvider represents a provider for the /connect UI (OpenCode style)
type ConnectProvider struct {
	ID          string
	Name        string
	Category    string // "Popular" or "Other"
	AuthType    string // "oauth", "api_key", "none"
	EnvVar      string // Environment variable for API key
	IsConnected bool
	AuthURL     string // URL for getting API key
	OAuthURL    string // OAuth authorization URL
	Priority    int    // Lower = higher priority in Popular
}

// GetConnectProviders returns all providers for /connect UI (OpenCode style)
func GetConnectProviders() []ConnectProvider {
	providers := []ConnectProvider{
		// Popular (우선순위 순)
		{ID: "anthropic", Name: "Anthropic", Category: "Popular", AuthType: "api_key", EnvVar: "ANTHROPIC_API_KEY", AuthURL: "https://console.anthropic.com/settings/keys", Priority: 1},
		{ID: "github-copilot", Name: "GitHub Copilot", Category: "Popular", AuthType: "oauth", EnvVar: "GITHUB_COPILOT_TOKEN", OAuthURL: "https://github.com/login/device", Priority: 2},
		{ID: "openai", Name: "OpenAI", Category: "Popular", AuthType: "api_key", EnvVar: "OPENAI_API_KEY", AuthURL: "https://platform.openai.com/api-keys", Priority: 3},
		{ID: "google", Name: "Google", Category: "Popular", AuthType: "api_key", EnvVar: "GOOGLE_API_KEY", AuthURL: "https://aistudio.google.com/apikey", Priority: 4},
		{ID: "ollama", Name: "Ollama (Local)", Category: "Popular", AuthType: "none", Priority: 5},

		// Other (알파벳 순)
		{ID: "302ai", Name: "302.AI", Category: "Other", AuthType: "api_key", EnvVar: "API_302AI_KEY", AuthURL: "https://302.ai"},
		{ID: "amazon-bedrock", Name: "Amazon Bedrock", Category: "Other", AuthType: "api_key", EnvVar: "AWS_ACCESS_KEY_ID", AuthURL: "https://aws.amazon.com/bedrock"},
		{ID: "azure", Name: "Azure OpenAI", Category: "Other", AuthType: "api_key", EnvVar: "AZURE_OPENAI_API_KEY", AuthURL: "https://portal.azure.com"},
		{ID: "baseten", Name: "Baseten", Category: "Other", AuthType: "api_key", EnvVar: "BASETEN_API_KEY", AuthURL: "https://baseten.co"},
		{ID: "cerebras", Name: "Cerebras", Category: "Other", AuthType: "api_key", EnvVar: "CEREBRAS_API_KEY", AuthURL: "https://cloud.cerebras.ai"},
		{ID: "chutes", Name: "Chutes", Category: "Other", AuthType: "api_key", EnvVar: "CHUTES_API_KEY", AuthURL: "https://chutes.ai"},
		{ID: "cloudflare", Name: "Cloudflare Workers AI", Category: "Other", AuthType: "api_key", EnvVar: "CLOUDFLARE_API_TOKEN", AuthURL: "https://dash.cloudflare.com"},
		{ID: "cohere", Name: "Cohere", Category: "Other", AuthType: "api_key", EnvVar: "COHERE_API_KEY", AuthURL: "https://dashboard.cohere.com/api-keys"},
		{ID: "cortecs", Name: "Cortecs", Category: "Other", AuthType: "api_key", EnvVar: "CORTECS_API_KEY", AuthURL: "https://cortecs.ai"},
		{ID: "deepseek", Name: "DeepSeek", Category: "Other", AuthType: "api_key", EnvVar: "DEEPSEEK_API_KEY", AuthURL: "https://platform.deepseek.com/api_keys"},
		{ID: "fastrouter", Name: "FastRouter", Category: "Other", AuthType: "api_key", EnvVar: "FASTROUTER_API_KEY", AuthURL: "https://fastrouter.ai"},
		{ID: "fireworks", Name: "Fireworks AI", Category: "Other", AuthType: "api_key", EnvVar: "FIREWORKS_API_KEY", AuthURL: "https://fireworks.ai"},
		{ID: "github-models", Name: "GitHub Models", Category: "Other", AuthType: "api_key", EnvVar: "GITHUB_TOKEN", AuthURL: "https://github.com/settings/tokens"},
		{ID: "groq", Name: "Groq", Category: "Other", AuthType: "api_key", EnvVar: "GROQ_API_KEY", AuthURL: "https://console.groq.com/keys"},
		{ID: "helicone", Name: "Helicone", Category: "Other", AuthType: "api_key", EnvVar: "HELICONE_API_KEY", AuthURL: "https://helicone.ai"},
		{ID: "huggingface", Name: "Hugging Face", Category: "Other", AuthType: "api_key", EnvVar: "HF_TOKEN", AuthURL: "https://huggingface.co/settings/tokens"},
		{ID: "inception", Name: "Inception", Category: "Other", AuthType: "api_key", EnvVar: "INCEPTION_API_KEY", AuthURL: "https://inception.ai"},
		{ID: "kimi", Name: "Kimi (Moonshot)", Category: "Other", AuthType: "api_key", EnvVar: "MOONSHOT_API_KEY", AuthURL: "https://platform.moonshot.cn/console/api-keys"},
		{ID: "llamacloud", Name: "Llama Cloud", Category: "Other", AuthType: "api_key", EnvVar: "LLAMA_CLOUD_API_KEY", AuthURL: "https://cloud.llama.com"},
		{ID: "minimax", Name: "MiniMax", Category: "Other", AuthType: "api_key", EnvVar: "MINIMAX_API_KEY", AuthURL: "https://minimax.chat"},
		{ID: "mistral", Name: "Mistral AI", Category: "Other", AuthType: "api_key", EnvVar: "MISTRAL_API_KEY", AuthURL: "https://console.mistral.ai/api-keys"},
		{ID: "moark", Name: "Moark", Category: "Other", AuthType: "api_key", EnvVar: "MOARK_API_KEY", AuthURL: "https://moark.ai"},
		{ID: "nvidia", Name: "NVIDIA NIM", Category: "Other", AuthType: "api_key", EnvVar: "NVIDIA_API_KEY", AuthURL: "https://build.nvidia.com"},
		{ID: "ollama-cloud", Name: "Ollama Cloud", Category: "Other", AuthType: "api_key", EnvVar: "OLLAMA_CLOUD_API_KEY", AuthURL: "https://ollama.com/cloud"},
		{ID: "openrouter", Name: "OpenRouter", Category: "Other", AuthType: "api_key", EnvVar: "OPENROUTER_API_KEY", AuthURL: "https://openrouter.ai/keys"},
		{ID: "perplexity", Name: "Perplexity", Category: "Other", AuthType: "api_key", EnvVar: "PERPLEXITY_API_KEY", AuthURL: "https://perplexity.ai/settings/api"},
		{ID: "replicate", Name: "Replicate", Category: "Other", AuthType: "api_key", EnvVar: "REPLICATE_API_TOKEN", AuthURL: "https://replicate.com/account/api-tokens"},
		{ID: "sambanova", Name: "SambaNova", Category: "Other", AuthType: "api_key", EnvVar: "SAMBANOVA_API_KEY", AuthURL: "https://sambanova.ai"},
		{ID: "siliconflow", Name: "SiliconFlow", Category: "Other", AuthType: "api_key", EnvVar: "SILICONFLOW_API_KEY", AuthURL: "https://siliconflow.cn"},
		{ID: "together", Name: "Together AI", Category: "Other", AuthType: "api_key", EnvVar: "TOGETHER_API_KEY", AuthURL: "https://api.together.ai/settings/api-keys"},
		{ID: "upstage", Name: "Upstage", Category: "Other", AuthType: "api_key", EnvVar: "UPSTAGE_API_KEY", AuthURL: "https://console.upstage.ai"},
		{ID: "vertex", Name: "Vertex AI", Category: "Other", AuthType: "api_key", EnvVar: "GOOGLE_APPLICATION_CREDENTIALS", AuthURL: "https://console.cloud.google.com/vertex-ai"},
		{ID: "vivgrid", Name: "Vivgrid", Category: "Other", AuthType: "api_key", EnvVar: "VIVGRID_API_KEY", AuthURL: "https://vivgrid.com"},
		{ID: "vultr", Name: "Vultr", Category: "Other", AuthType: "api_key", EnvVar: "VULTR_API_KEY", AuthURL: "https://my.vultr.com/settings/#settingsapi"},
		{ID: "wandb", Name: "Weights & Biases", Category: "Other", AuthType: "api_key", EnvVar: "WANDB_API_KEY", AuthURL: "https://wandb.ai/authorize"},
		{ID: "xai", Name: "xAI (Grok)", Category: "Other", AuthType: "api_key", EnvVar: "XAI_API_KEY", AuthURL: "https://console.x.ai"},
	}

	// Check connection status for each provider
	for i := range providers {
		providers[i].IsConnected = checkProviderConnected(&providers[i])
	}

	return providers
}

// checkProviderConnected checks if a provider is connected
func checkProviderConnected(p *ConnectProvider) bool {
	// First check auth store
	store := auth.GetStore()
	if store.IsConnected(p.ID) {
		return true
	}

	switch p.AuthType {
	case "none":
		// For Ollama, check if server is running
		if p.ID == "ollama" {
			return checkOllamaAvailable()
		}
		return true
	case "api_key":
		return os.Getenv(p.EnvVar) != ""
	case "oauth":
		// Check for OAuth tokens in env
		return os.Getenv(p.EnvVar) != ""
	}
	return false
}

// FilterConnectProviders filters providers by search query
func FilterConnectProviders(providers []ConnectProvider, query string) []ConnectProvider {
	if query == "" {
		return providers
	}

	query = strings.ToLower(query)
	var filtered []ConnectProvider
	for _, p := range providers {
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.ID), query) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// ProviderInfo holds provider information for the TUI
type ProviderInfo struct {
	Name        string
	DisplayName string
	Kind        string // "local", "cloud"
	AuthType    string // "none", "api_key", "oauth"
	Models      []ModelInfo
	IsAvailable bool
	AuthStatus  string // "authenticated", "not_authenticated"
}

// ModelInfo holds model information
type ModelInfo struct {
	ID          string
	Name        string
	Provider    string
	Size        string
	Description string
	IsInstalled bool
	IsPrimary   bool
}

// GetAvailableProviders returns all available providers with their status
func GetAvailableProviders() []ProviderInfo {
	providers := []ProviderInfo{
		{
			Name:        "ollama",
			DisplayName: "Ollama (Local)",
			Kind:        "local",
			AuthType:    "none",
			IsAvailable: checkOllamaAvailable(),
			AuthStatus:  "authenticated", // Local doesn't need auth
		},
		{
			Name:        "anthropic",
			DisplayName: "Anthropic (Claude)",
			Kind:        "cloud",
			AuthType:    "api_key",
			IsAvailable: true,
			AuthStatus:  checkAPIKeyStatus("ANTHROPIC_API_KEY"),
		},
		{
			Name:        "openai",
			DisplayName: "OpenAI (GPT)",
			Kind:        "cloud",
			AuthType:    "api_key",
			IsAvailable: true,
			AuthStatus:  checkAPIKeyStatus("OPENAI_API_KEY"),
		},
		{
			Name:        "google",
			DisplayName: "Google (Gemini)",
			Kind:        "cloud",
			AuthType:    "api_key",
			IsAvailable: true,
			AuthStatus:  checkAPIKeyStatus("GOOGLE_API_KEY"),
		},
		{
			Name:        "openrouter",
			DisplayName: "OpenRouter",
			Kind:        "cloud",
			AuthType:    "api_key",
			IsAvailable: true,
			AuthStatus:  checkAPIKeyStatus("OPENROUTER_API_KEY"),
		},
		{
			Name:        "groq",
			DisplayName: "Groq",
			Kind:        "cloud",
			AuthType:    "api_key",
			IsAvailable: true,
			AuthStatus:  checkAPIKeyStatus("GROQ_API_KEY"),
		},
	}

	// Load models for Ollama if available
	if providers[0].IsAvailable {
		providers[0].Models = getOllamaModels()
	}

	return providers
}

// GetModelsForProvider returns available models for a provider
func GetModelsForProvider(providerName string) []ModelInfo {
	switch providerName {
	case "ollama":
		return getOllamaModels()
	case "anthropic":
		return []ModelInfo{
			{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", Description: "Latest Claude model", IsPrimary: true},
			{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Provider: "anthropic", Description: "Fast and efficient"},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic", Description: "Most capable"},
		}
	case "openai":
		return []ModelInfo{
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Description: "Latest GPT-4", IsPrimary: true},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", Description: "Fast and affordable"},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", Description: "Previous generation"},
			{ID: "o1", Name: "o1", Provider: "openai", Description: "Reasoning model"},
		}
	case "google":
		return []ModelInfo{
			{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: "google", Description: "Fast multimodal", IsPrimary: true},
			{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Provider: "google", Description: "High capability"},
			{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Provider: "google", Description: "Fast responses"},
		}
	case "openrouter":
		return []ModelInfo{
			{ID: "anthropic/claude-sonnet-4", Name: "Claude Sonnet 4", Provider: "openrouter", IsPrimary: true},
			{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter"},
			{ID: "google/gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: "openrouter"},
			{ID: "meta-llama/llama-3.3-70b", Name: "Llama 3.3 70B", Provider: "openrouter"},
		}
	case "groq":
		return []ModelInfo{
			{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Provider: "groq", IsPrimary: true},
			{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Provider: "groq"},
		}
	default:
		return nil
	}
}

// EssentialOllamaModels returns lightweight but high-performance models (2025-2026 기준)
var EssentialOllamaModels = []ModelInfo{
	// 🥇 가성비 최강
	{ID: "qwen2.5:7b", Name: "Qwen 2.5 7B", Provider: "ollama", Size: "4.5GB", Description: "🥇 동급 대비 지능 매우 높음", IsPrimary: true},
	{ID: "qwen2.5:3b", Name: "Qwen 2.5 3B", Provider: "ollama", Size: "2GB", Description: "빠른 범용"},

	// 🥈 저사양 최강
	{ID: "phi3:mini", Name: "Phi-3 Mini", Provider: "ollama", Size: "1.3GB", Description: "🥈 저사양 최강, 추론력 우수"},
	{ID: "phi4", Name: "Phi-4", Provider: "ollama", Size: "8GB", Description: "추론 특화"},

	// 🥉 개발용
	{ID: "qwen2.5-coder:7b", Name: "Qwen 2.5 Coder 7B", Provider: "ollama", Size: "4.5GB", Description: "🥉 저사양 대비 코딩 성능 최고"},
	{ID: "qwen2.5-coder:3b", Name: "Qwen 2.5 Coder 3B", Provider: "ollama", Size: "2GB", Description: "빠른 코드 생성"},
	{ID: "codegemma:7b", Name: "CodeGemma 7B", Provider: "ollama", Size: "4.5GB", Description: "Java/JS/Python 강함"},
	{ID: "deepseek-coder:6.7b", Name: "DeepSeek Coder 6.7B", Provider: "ollama", Size: "4GB", Description: "코드 전문"},

	// 🏆 밸런스 표준
	{ID: "llama3.1:8b", Name: "Llama 3.1 8B", Provider: "ollama", Size: "4.7GB", Description: "🏆 밸런스 표준, 도구 연동 좋음"},

	// 초경량
	{ID: "gemma:2b", Name: "Gemma 2B", Provider: "ollama", Size: "1.4GB", Description: "안정성 + 속도"},
	{ID: "tinyllama:latest", Name: "TinyLlama", Provider: "ollama", Size: "0.6GB", Description: "Ultra-fast responses"},
	{ID: "qwen2.5:1.5b", Name: "Qwen 2.5 1.5B", Provider: "ollama", Size: "1GB", Description: "초경량 + 높은 성능"},

	// 임베딩
	{ID: "nomic-embed-text:latest", Name: "Nomic Embed Text", Provider: "ollama", Size: "0.3GB", Description: "Text embeddings for RAG"},
}

func checkOllamaAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	host := os.Getenv("DEVORCH_OLLAMA_HOST")
	if host == "" {
		host = "http://127.0.0.1:11434"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", host+"/api/tags", nil)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func checkAPIKeyStatus(envVar string) string {
	if os.Getenv(envVar) != "" {
		return "authenticated"
	}
	return "not_authenticated"
}

func getOllamaModels() []ModelInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	host := os.Getenv("DEVORCH_OLLAMA_HOST")
	if host == "" {
		host = "http://127.0.0.1:11434"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", host+"/api/tags", nil)
	if err != nil {
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	// Mark essential models as installed
	installedSet := make(map[string]bool)
	for _, m := range result.Models {
		baseName := strings.Split(m.Name, ":")[0]
		installedSet[m.Name] = true
		installedSet[baseName] = true
	}

	var models []ModelInfo
	for _, m := range result.Models {
		models = append(models, ModelInfo{
			ID:          m.Name,
			Name:        m.Name,
			Provider:    "ollama",
			Size:        formatSize(m.Size),
			IsInstalled: true,
		})
	}

	// Add essential models that aren't installed
	for _, essential := range EssentialOllamaModels {
		if !installedSet[essential.ID] {
			essential.IsInstalled = false
			models = append(models, essential)
		}
	}

	return models
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// OllamaClient handles Ollama API calls
type OllamaClient struct {
	host string
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient() *OllamaClient {
	host := os.Getenv("DEVORCH_OLLAMA_HOST")
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	return &OllamaClient{host: host}
}

// Chat sends a chat request to Ollama (non-streaming)
func (c *OllamaClient) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	type ollamaMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type chatRequest struct {
		Model    string          `json:"model"`
		Messages []ollamaMessage `json:"messages"`
		Stream   bool            `json:"stream"`
	}

	ollamaMsgs := make([]ollamaMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role != "system" || m.Content != "" {
			ollamaMsgs = append(ollamaMsgs, ollamaMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	reqBody := chatRequest{
		Model:    model,
		Messages: ollamaMsgs,
		Stream:   false,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Message.Content, nil
}

// ChatStream sends a streaming chat request to Ollama
func (c *OllamaClient) ChatStream(ctx context.Context, model string, messages []Message, onChunk func(string)) error {
	type ollamaMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type chatRequest struct {
		Model    string          `json:"model"`
		Messages []ollamaMessage `json:"messages"`
		Stream   bool            `json:"stream"`
	}

	ollamaMsgs := make([]ollamaMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role != "system" || m.Content != "" {
			ollamaMsgs = append(ollamaMsgs, ollamaMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	reqBody := chatRequest{
		Model:    model,
		Messages: ollamaMsgs,
		Stream:   true,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// Stream response
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}

		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			onChunk(chunk.Message.Content)
		}

		if chunk.Done {
			break
		}
	}

	return scanner.Err()
}

// PullModel downloads a model
func (c *OllamaClient) PullModel(ctx context.Context, model string) error {
	reqBody := struct {
		Name string `json:"name"`
	}{Name: model}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/api/pull", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// No timeout for pull - it can take a long time
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("pull returned status %d", resp.StatusCode)
	}

	return nil
}

// OpenBrowserForLogin opens the browser for OAuth login
func OpenBrowserForLogin(provider string) error {
	var url string

	switch provider {
	case "anthropic":
		url = "https://console.anthropic.com/settings/keys"
	case "openai":
		url = "https://platform.openai.com/api-keys"
	case "google":
		url = "https://aistudio.google.com/app/apikey"
	case "github":
		url = "https://github.com/settings/tokens"
	case "openrouter":
		url = "https://openrouter.ai/keys"
	case "groq":
		url = "https://console.groq.com/keys"
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}

	return openURL(url)
}

func openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

// InstallEssentialModels installs essential Ollama models
func InstallEssentialModels(progressCh chan<- string) error {
	client := NewOllamaClient()

	// Check which models are already installed
	installed := getOllamaModels()
	installedSet := make(map[string]bool)
	for _, m := range installed {
		if m.IsInstalled {
			installedSet[m.ID] = true
		}
	}

	// Install missing essential models
	for _, model := range EssentialOllamaModels {
		if installedSet[model.ID] {
			if progressCh != nil {
				progressCh <- fmt.Sprintf("✓ %s (already installed)", model.ID)
			}
			continue
		}

		if progressCh != nil {
			progressCh <- fmt.Sprintf("⬇ Downloading %s (%s)...", model.ID, model.Size)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		err := client.PullModel(ctx, model.ID)
		cancel()

		if err != nil {
			if progressCh != nil {
				progressCh <- fmt.Sprintf("✗ Failed to install %s: %v", model.ID, err)
			}
			continue
		}

		if progressCh != nil {
			progressCh <- fmt.Sprintf("✓ %s installed", model.ID)
		}
	}

	return nil
}

// ===============================================
// Cloud Provider Clients
// ===============================================

// AnthropicClient handles Anthropic API calls
type AnthropicClient struct {
	apiKey string
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient() *AnthropicClient {
	return &AnthropicClient{
		apiKey: os.Getenv("ANTHROPIC_API_KEY"),
	}
}

// Chat sends a chat request to Anthropic
func (c *AnthropicClient) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	type anthropicMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type chatRequest struct {
		Model     string             `json:"model"`
		MaxTokens int                `json:"max_tokens"`
		Messages  []anthropicMessage `json:"messages"`
	}

	var anthropicMsgs []anthropicMessage
	for _, m := range messages {
		if m.Role == "system" {
			continue // Anthropic handles system differently
		}
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := chatRequest{
		Model:     model,
		MaxTokens: 4096,
		Messages:  anthropicMsgs,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", fmt.Errorf("empty response from anthropic")
}

// OpenAIClient handles OpenAI API calls
type OpenAIClient struct {
	apiKey  string
	baseURL string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient() *OpenAIClient {
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIClient{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		baseURL: baseURL,
	}
}

// Chat sends a chat request to OpenAI
func (c *OpenAIClient) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	type openaiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type chatRequest struct {
		Model    string          `json:"model"`
		Messages []openaiMessage `json:"messages"`
	}

	var openaiMsgs []openaiMessage
	for _, m := range messages {
		openaiMsgs = append(openaiMsgs, openaiMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := chatRequest{
		Model:    model,
		Messages: openaiMsgs,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("empty response from openai")
}

// GoogleClient handles Google AI API calls
type GoogleClient struct {
	apiKey string
}

// NewGoogleClient creates a new Google AI client
func NewGoogleClient() *GoogleClient {
	return &GoogleClient{
		apiKey: os.Getenv("GOOGLE_API_KEY"),
	}
}

// Chat sends a chat request to Google AI
func (c *GoogleClient) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("GOOGLE_API_KEY not set")
	}

	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}
	type chatRequest struct {
		Contents []content `json:"contents"`
	}

	var contents []content
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "system" {
			role = "user" // Google doesn't have system role
		}
		contents = append(contents, content{
			Role:  role,
			Parts: []part{{Text: m.Content}},
		})
	}

	reqBody := chatRequest{Contents: contents}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("google request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("google returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("empty response from google")
}

// OpenRouterClient handles OpenRouter API calls (uses OpenAI-compatible API)
type OpenRouterClient struct {
	apiKey string
}

// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient() *OpenRouterClient {
	return &OpenRouterClient{
		apiKey: os.Getenv("OPENROUTER_API_KEY"),
	}
}

// Chat sends a chat request to OpenRouter
func (c *OpenRouterClient) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	type openaiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type chatRequest struct {
		Model    string          `json:"model"`
		Messages []openaiMessage `json:"messages"`
	}

	var openaiMsgs []openaiMessage
	for _, m := range messages {
		openaiMsgs = append(openaiMsgs, openaiMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := chatRequest{
		Model:    model,
		Messages: openaiMsgs,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://devorch.dev")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openrouter returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("empty response from openrouter")
}

// GroqClient handles Groq API calls (uses OpenAI-compatible API)
type GroqClient struct {
	apiKey string
}

// NewGroqClient creates a new Groq client
func NewGroqClient() *GroqClient {
	return &GroqClient{
		apiKey: os.Getenv("GROQ_API_KEY"),
	}
}

// Chat sends a chat request to Groq
func (c *GroqClient) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY not set")
	}

	type openaiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type chatRequest struct {
		Model    string          `json:"model"`
		Messages []openaiMessage `json:"messages"`
	}

	var openaiMsgs []openaiMessage
	for _, m := range messages {
		openaiMsgs = append(openaiMsgs, openaiMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := chatRequest{
		Model:    model,
		Messages: openaiMsgs,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("empty response from groq")
}

// ChatWithProvider sends a message to the specified provider
func ChatWithProvider(ctx context.Context, provider, model string, messages []Message) (string, error) {
	switch provider {
	case "ollama":
		return NewOllamaClient().Chat(ctx, model, messages)
	case "anthropic":
		return NewAnthropicClient().Chat(ctx, model, messages)
	case "openai":
		return NewOpenAIClient().Chat(ctx, model, messages)
	case "google":
		return NewGoogleClient().Chat(ctx, model, messages)
	case "openrouter":
		return NewOpenRouterClient().Chat(ctx, model, messages)
	case "groq":
		return NewGroqClient().Chat(ctx, model, messages)
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}
