// Package autosetup provides automatic hardware detection and model setup.
package autosetup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"devorch/internal/global"
)

// Result holds the result of auto setup.
type Result struct {
	Profile          HWInfo
	Tier             string
	RecommendedModel string
	OllamaInstalled  bool
	ModelInstalled   bool
}

// HWInfo holds hardware information.
type HWInfo struct {
	OS       string
	Arch     string
	MemoryGB int
	HasGPU   bool
	GPUKind  string
}

// ModelRecommendation maps tiers to recommended models across all verified providers.
// Updated for complete AI OS support with all working providers.
var ModelRecommendation = map[string][]string{
	// Ultra tier: 64GB+ RAM, NVIDIA 24GB+ - AI OS Orchestrator Tier
	"ultra": {
		// Direct API access (verified working)
		"openai/gpt-5.2", "openai/gpt-5.2-pro", "openai/o3",
		// OpenRouter access to latest models (verified working)
		"openrouter/anthropic/claude-opus-4.5", "openrouter/anthropic/claude-sonnet-4.5",
		"openrouter/openai/gpt-5.2", "openrouter/openai/o3",
		// Local powerhouse models
		"qwen2.5:32b", "llama3.1:70b", "deepseek-coder:33b",
	},
	// High tier: 24GB+ RAM, GPU available - AI OS Senior Developer Tier
	"high": {
		// OpenAI direct (verified working)
		"openai/gpt-4o", "openai/gpt-4.1", "openai/o1",
		// OpenRouter Claude access (verified working)
		"openrouter/anthropic/claude-3.7-sonnet", "openrouter/anthropic/claude-3.5-sonnet",
		// OpenRouter other top models
		"openrouter/openai/gpt-4o", "openrouter/qwen/qwen3-max",
		"openrouter/x-ai/grok-4", "openrouter/mistralai/mistral-large-2512",
		// Local high-end models
		"qwen2.5:14b", "qwen2.5-coder:14b", "llama3.1:8b",
	},
	// Mid tier: 16GB RAM, integrated/small GPU - AI OS Regular Developer Tier
	"mid": {
		// OpenAI efficient models
		"openai/gpt-4o-mini", "openai/gpt-3.5-turbo",
		// OpenRouter efficient access
		"openrouter/openai/gpt-4o-mini", "openrouter/qwen/qwen-2.5-72b-instruct",
		"openrouter/meta-llama/llama-3.1-70b-instruct",
		"openrouter/anthropic/claude-3.5-haiku",
		// Local optimized models
		"qwen2.5:7b", "qwen2.5-coder:7b", "llama3.1:8b", "phi4",
	},
	// Low tier: 8-12GB RAM, CPU only - AI OS Background Worker Tier
	"low": {
		// Cost-effective OpenAI
		"openai/gpt-4o-mini",
		// OpenRouter free tiers (verified working)
		"openrouter/qwen/qwen-2.5-7b-instruct:free",
		"openrouter/meta-llama/llama-3.2-3b-instruct:free",
		"openrouter/google/gemma-3-4b-it:free",
		// Local lightweight models
		"qwen2.5:3b", "phi3:mini", "qwen2.5-coder:3b", "gemma:2b",
	},
	// Minimal tier: <8GB RAM - AI OS Essential Operations
	"minimal": {
		// OpenRouter ultra-light free models
		"openrouter/qwen/qwen-2.5-7b-instruct:free",
		"openrouter/google/gemma-3-4b-it:free",
		// Ultra-light local models
		"phi3:mini", "qwen2.5:1.5b", "tinyllama", "gemma:2b",
	},
}

// Run performs automatic setup.
func Run() (*Result, error) {
	fmt.Println("🔍 Detecting hardware...")

	profile, err := global.GetHWProfile()
	if err != nil {
		return nil, fmt.Errorf("failed to detect hardware: %w", err)
	}

	tier := profile.Tier()
	memGB := int(profile.MemTotalMB / 1024)

	result := &Result{
		Profile: HWInfo{
			OS:       profile.OS,
			Arch:     profile.Arch,
			MemoryGB: memGB,
			HasGPU:   profile.HasAccel,
			GPUKind:  profile.AccelKind,
		},
		Tier: tier,
	}

	fmt.Printf("  ✓ OS: %s (%s)\n", profile.OS, profile.Arch)
	fmt.Printf("  ✓ Memory: %d GB\n", memGB)
	if profile.HasAccel {
		fmt.Printf("  ✓ GPU: %s\n", profile.AccelKind)
	} else {
		fmt.Println("  ✓ GPU: None detected")
	}
	fmt.Printf("  ✓ Tier: %s\n", tier)
	fmt.Println()

	// Select recommended model
	models := ModelRecommendation[tier]
	if len(models) == 0 {
		models = ModelRecommendation["low"]
	}
	result.RecommendedModel = models[0]
	fmt.Printf("📦 Recommended model: %s\n", result.RecommendedModel)
	fmt.Println()

	// Check Ollama installation
	fmt.Println("🔍 Checking Ollama...")
	ollamaInstalled := checkOllamaInstalled()
	result.OllamaInstalled = ollamaInstalled

	if !ollamaInstalled {
		fmt.Println("  ⚠ Ollama not found")
		fmt.Println("  Installing Ollama...")
		if err := installOllama(); err != nil {
			fmt.Printf("  ✗ Failed to install Ollama: %v\n", err)
			fmt.Println("  Please install manually: https://ollama.ai/download")
		} else {
			fmt.Println("  ✓ Ollama installed")
			result.OllamaInstalled = true
		}
	} else {
		fmt.Println("  ✓ Ollama installed")
	}
	fmt.Println()

	// Check/install recommended model
	if result.OllamaInstalled {
		fmt.Printf("🤖 Checking model: %s...\n", result.RecommendedModel)
		if checkModelInstalled(result.RecommendedModel) {
			fmt.Printf("  ✓ Model %s already installed\n", result.RecommendedModel)
			result.ModelInstalled = true
		} else {
			fmt.Printf("  ⬇ Model not found. Pull with: ollama pull %s\n", result.RecommendedModel)
			// Optionally auto-pull (can be slow)
			if os.Getenv("DEVORCH_AUTO_INSTALL") == "1" {
				fmt.Printf("  Installing %s (this may take a while)...\n", result.RecommendedModel)
				if err := pullModel(result.RecommendedModel); err != nil {
					fmt.Printf("  ✗ Failed to pull model: %v\n", err)
				} else {
					fmt.Println("  ✓ Model installed")
					result.ModelInstalled = true
				}
			}
		}
	}
	fmt.Println()

	return result, nil
}

// checkOllamaInstalled checks if Ollama is installed.
func checkOllamaInstalled() bool {
	cmd := exec.Command("ollama", "--version")
	err := cmd.Run()
	return err == nil
}

// installOllama installs Ollama based on OS.
func installOllama() error {
	switch runtime.GOOS {
	case "darwin":
		// Try Homebrew first
		cmd := exec.Command("brew", "install", "ollama")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
		// Fallback to curl installer
		cmd = exec.Command("sh", "-c", "curl -fsSL https://ollama.ai/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "linux":
		cmd := exec.Command("sh", "-c", "curl -fsSL https://ollama.ai/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "windows":
		// Windows requires manual download or winget
		return fmt.Errorf("please install Ollama from https://ollama.ai/download")

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// checkModelInstalled checks if a model is installed in Ollama.
func checkModelInstalled(model string) bool {
	cmd := exec.Command("ollama", "list")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// Model name might be in format "name:tag" or just "name"
	modelBase := strings.Split(model, ":")[0]
	return strings.Contains(string(out), modelBase)
}

// IsModelInstalled is exported version of checkModelInstalled
func IsModelInstalled(model string) bool {
	return checkModelInstalled(model)
}

// pullModel pulls a model using Ollama.
func pullModel(model string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ollama", "pull", model)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetRecommendedModel returns the recommended model for the current hardware.
func GetRecommendedModel() (string, error) {
	profile, err := global.GetHWProfile()
	if err != nil {
		return "", err
	}

	tier := profile.Tier()
	models := ModelRecommendation[tier]
	if len(models) == 0 {
		return "phi3:mini", nil
	}
	return models[0], nil
}

// QuickCheck performs a quick hardware check and returns tier + recommended model.
func QuickCheck() (tier, model string, err error) {
	profile, err := global.GetHWProfile()
	if err != nil {
		return "", "", err
	}

	tier = profile.Tier()
	models := ModelRecommendation[tier]
	if len(models) == 0 {
		return tier, "phi3:mini", nil
	}
	return tier, models[0], nil
}

// EssentialModels defines lightweight but powerful models for each purpose (2025-2026 기준)
var EssentialModels = map[string][]string{
	// General purpose - balanced performance (가성비 최강)
	"general": {
		"qwen2.5:7b", // 🥇 동급 대비 지능 매우 높음 (4.5GB)
		"qwen2.5:3b", // 빠른 범용 (2GB)
		"phi3:mini",  // 🥈 저사양 최강, 추론력 우수 (1.3GB)
		"gemma:2b",   // 안정성 + 속도 (1.4GB)
	},
	// Code assistance (개발용)
	"code": {
		"qwen2.5-coder:7b",    // 🥉 저사양 대비 코딩 성능 최고 (4.5GB)
		"qwen2.5-coder:3b",    // 빠른 코드 생성 (2GB)
		"deepseek-coder:6.7b", // 코드 전문 (4GB)
		"codegemma:7b",        // Java/JS/Python 강함 (4.5GB)
	},
	// Fast responses (초경량)
	"fast": {
		"tinyllama:latest", // Ultra-fast, minimal (0.6GB)
		"qwen2.5:1.5b",     // 초경량 + 높은 성능 (1GB)
		"gemma:2b",         // 요약/분류 (1.4GB)
	},
	// Reasoning (추론 특화)
	"reasoning": {
		"phi4",        // Microsoft's 추론 특화 (8GB)
		"llama3.1:8b", // 🏆 밸런스 표준 (4.7GB)
	},
	// Text embeddings for RAG
	"embedding": {
		"nomic-embed-text:latest", // Text embeddings (0.3GB)
	},
}

// GetEssentialModels returns essential models based on tier (2025-2026 저사양 최적화)
func GetEssentialModels(tier string) []string {
	var models []string

	switch tier {
	case "ultra", "high":
		// High-end: 전체 모델 세트
		models = append(models,
			"qwen2.5:7b",       // 일반용 (가성비 최강)
			"qwen2.5-coder:7b", // 코딩용 (개발자 필수)
			"llama3.1:8b",      // 밸런스
			"phi4",             // 추론 특화
		)
		models = append(models, EssentialModels["embedding"]...)
	case "mid":
		// Mid-tier: 밸런스 선택
		models = append(models,
			"qwen2.5:7b",       // 🥇 일반용 (가성비 최강)
			"qwen2.5-coder:7b", // 🥉 코딩용
			"phi3:mini",        // 빠른 응답
		)
		models = append(models, EssentialModels["embedding"]...)
	case "low":
		// Low-tier: 경량 모델만
		models = append(models,
			"qwen2.5:3b",       // 일반용
			"phi3:mini",        // 🥈 저사양 최강
			"qwen2.5-coder:3b", // 코딩용
			"gemma:2b",         // 요약/분류
		)
		models = append(models, EssentialModels["embedding"]...)
	default:
		// Minimal: 필수 최소만
		models = append(models,
			"phi3:mini",        // 저사양 최강 (1.3GB)
			"qwen2.5:1.5b",     // 초경량 (1GB)
			"tinyllama:latest", // 가장 작음 (0.6GB)
		)
		models = append(models, EssentialModels["embedding"]...)
	}

	return models
}

// InstallEssentialModels installs essential models for the system
func InstallEssentialModels(verbose bool) error {
	profile, err := global.GetHWProfile()
	if err != nil {
		return fmt.Errorf("failed to detect hardware: %w", err)
	}

	tier := profile.Tier()
	models := GetEssentialModels(tier)

	fmt.Printf("📦 Installing %d essential models for %s tier...\n\n", len(models), tier)

	installed := 0
	failed := 0

	for i, model := range models {
		// Check if already installed
		if checkModelInstalled(model) {
			fmt.Printf("  [%d/%d] ✓ %s (already installed)\n", i+1, len(models), model)
			installed++
			continue
		}

		fmt.Printf("  [%d/%d] ⬇ Downloading %s...\n", i+1, len(models), model)

		if err := pullModel(model); err != nil {
			fmt.Printf("  [%d/%d] ✗ Failed to install %s: %v\n", i+1, len(models), model, err)
			failed++
			continue
		}

		fmt.Printf("  [%d/%d] ✓ %s installed\n", i+1, len(models), model)
		installed++
	}

	fmt.Printf("\n📊 Installation Summary:\n")
	fmt.Printf("  ✓ Installed: %d\n", installed)
	if failed > 0 {
		fmt.Printf("  ✗ Failed: %d\n", failed)
	}

	return nil
}

// FullSetup performs complete setup including models and MCP
func FullSetup() error {
	fmt.Println("🚀 DevOrch Full Setup")
	fmt.Println()

	// Step 1: Hardware detection
	result, err := Run()
	if err != nil {
		return err
	}

	// Step 2: Install essential models (if auto-install enabled)
	if os.Getenv("DEVORCH_AUTO_INSTALL") == "1" && result.OllamaInstalled {
		fmt.Println("\n📦 Installing essential models...")
		if err := InstallEssentialModels(true); err != nil {
			fmt.Printf("  ⚠ Model installation had issues: %v\n", err)
		}
	} else if result.OllamaInstalled {
		fmt.Println("\n💡 To install essential models, run:")
		fmt.Println("   DEVORCH_AUTO_INSTALL=1 devorch setup")
		fmt.Println("\n   Or manually install recommended models:")
		for _, model := range GetEssentialModels(result.Tier) {
			fmt.Printf("   ollama pull %s\n", model)
		}
	}

	// Step 3: MCP setup guidance
	fmt.Println("\n🔌 MCP Server Setup")
	fmt.Println("  Essential MCP servers for DevOrch:")
	fmt.Println("  • filesystem - File system operations")
	fmt.Println("  • github - GitHub API integration")
	fmt.Println("  • fetch - HTTP fetch operations")
	fmt.Println("  • memory - Persistent memory storage")
	fmt.Println("\n  Run 'devorch mcp setup' to configure MCP servers")

	return nil
}
