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

// ModelRecommendation maps tiers to recommended models.
var ModelRecommendation = map[string][]string{
	"ultra":   {"llama3.3:70b", "deepseek-coder:33b", "qwen2.5:32b"},
	"high":    {"llama3.2:13b", "codellama:13b", "qwen2.5-coder:14b"},
	"mid":     {"llama3.2:7b", "qwen2.5-coder:7b", "deepseek-coder:6.7b"},
	"low":     {"llama3.2:3b", "phi3:medium", "qwen2.5-coder:3b"},
	"minimal": {"phi3:mini", "qwen2.5-coder:1.5b", "tinyllama"},
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
