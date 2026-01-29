// Package autosetup provides automatic hardware detection and model setup.
package autosetup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SystemProfile holds comprehensive system information.
type SystemProfile struct {
	OS           string       `json:"os"`
	Arch         string       `json:"arch"`
	MemoryGB     int          `json:"memory_gb"`
	CPUCores     int          `json:"cpu_cores"`
	GPU          GPUInfo      `json:"gpu"`
	Tier         string       `json:"tier"`
	OllamaStatus OllamaStatus `json:"ollama"`
}

// GPUInfo holds GPU information.
type GPUInfo struct {
	Available bool   `json:"available"`
	Type      string `json:"type"` // "nvidia", "amd", "apple", "intel", "none"
	Name      string `json:"name"`
	VRAM      int    `json:"vram_gb"` // In GB
}

// OllamaStatus holds Ollama installation status.
type OllamaStatus struct {
	Installed       bool     `json:"installed"`
	Running         bool     `json:"running"`
	Version         string   `json:"version"`
	InstalledModels []string `json:"installed_models"`
}

// ModelSpec defines a downloadable model.
type ModelSpec struct {
	Name        string  `json:"name"`
	Tag         string  `json:"tag"`
	SizeGB      float64 `json:"size_gb"`
	MinRAMGB    int     `json:"min_ram_gb"`
	MinVRAMGB   int     `json:"min_vram_gb"`
	Category    string  `json:"category"` // "general", "code", "fast", "reasoning"
	Description string  `json:"description"`
	Priority    int     `json:"priority"` // Lower is higher priority
}

// AvailableModels returns all available models for download.
var AvailableModels = []ModelSpec{
	// Ultra tier (64GB+ RAM or 24GB+ VRAM)
	{Name: "llama3.3", Tag: "70b", SizeGB: 40, MinRAMGB: 64, MinVRAMGB: 24, Category: "general", Description: "Most capable open model", Priority: 1},
	{Name: "deepseek-coder", Tag: "33b", SizeGB: 19, MinRAMGB: 48, MinVRAMGB: 16, Category: "code", Description: "Large coding model", Priority: 2},
	{Name: "qwen2.5", Tag: "32b", SizeGB: 18, MinRAMGB: 48, MinVRAMGB: 16, Category: "general", Description: "Qwen 2.5 large", Priority: 3},
	{Name: "mixtral", Tag: "8x7b", SizeGB: 26, MinRAMGB: 48, MinVRAMGB: 16, Category: "general", Description: "Mixture of experts", Priority: 4},

	// High tier (32GB+ RAM or 12GB+ VRAM)
	{Name: "llama3.2", Tag: "13b", SizeGB: 8, MinRAMGB: 16, MinVRAMGB: 8, Category: "general", Description: "Balanced general model", Priority: 5},
	{Name: "codellama", Tag: "13b", SizeGB: 8, MinRAMGB: 16, MinVRAMGB: 8, Category: "code", Description: "Code-specialized Llama", Priority: 6},
	{Name: "qwen2.5-coder", Tag: "14b", SizeGB: 8, MinRAMGB: 16, MinVRAMGB: 8, Category: "code", Description: "Qwen coding model", Priority: 7},
	{Name: "deepseek-coder", Tag: "6.7b", SizeGB: 4, MinRAMGB: 12, MinVRAMGB: 6, Category: "code", Description: "Fast coding model", Priority: 8},

	// Mid tier (16GB+ RAM or 8GB+ VRAM)
	{Name: "llama3.2", Tag: "7b", SizeGB: 4, MinRAMGB: 8, MinVRAMGB: 4, Category: "general", Description: "Fast general model", Priority: 9},
	{Name: "qwen2.5-coder", Tag: "7b", SizeGB: 4, MinRAMGB: 8, MinVRAMGB: 4, Category: "code", Description: "Fast coding model", Priority: 10},
	{Name: "mistral", Tag: "7b", SizeGB: 4, MinRAMGB: 8, MinVRAMGB: 4, Category: "general", Description: "Efficient general model", Priority: 11},
	{Name: "gemma2", Tag: "9b", SizeGB: 5, MinRAMGB: 10, MinVRAMGB: 5, Category: "general", Description: "Google's Gemma 2", Priority: 12},

	// Low tier (8GB+ RAM)
	{Name: "llama3.2", Tag: "3b", SizeGB: 2, MinRAMGB: 6, MinVRAMGB: 3, Category: "general", Description: "Compact general model", Priority: 13},
	{Name: "phi3", Tag: "medium", SizeGB: 2, MinRAMGB: 6, MinVRAMGB: 3, Category: "general", Description: "Microsoft Phi-3", Priority: 14},
	{Name: "qwen2.5-coder", Tag: "3b", SizeGB: 2, MinRAMGB: 6, MinVRAMGB: 3, Category: "code", Description: "Compact coding model", Priority: 15},
	{Name: "starcoder2", Tag: "3b", SizeGB: 2, MinRAMGB: 6, MinVRAMGB: 3, Category: "code", Description: "StarCoder 2 small", Priority: 16},

	// Minimal tier (4GB+ RAM)
	{Name: "phi3", Tag: "mini", SizeGB: 1.3, MinRAMGB: 4, MinVRAMGB: 2, Category: "general", Description: "Tiny but capable", Priority: 17},
	{Name: "qwen2.5-coder", Tag: "1.5b", SizeGB: 0.9, MinRAMGB: 4, MinVRAMGB: 2, Category: "code", Description: "Smallest coding model", Priority: 18},
	{Name: "tinyllama", Tag: "latest", SizeGB: 0.6, MinRAMGB: 3, MinVRAMGB: 1, Category: "fast", Description: "Ultra-fast responses", Priority: 19},
	{Name: "nomic-embed-text", Tag: "latest", SizeGB: 0.3, MinRAMGB: 2, MinVRAMGB: 1, Category: "embedding", Description: "Text embeddings", Priority: 20},
}

// DetectSystem performs comprehensive system detection.
func DetectSystem() (*SystemProfile, error) {
	profile := &SystemProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCores: runtime.NumCPU(),
	}

	// Detect memory
	profile.MemoryGB = detectMemory()

	// Detect GPU
	profile.GPU = detectGPU()

	// Calculate tier
	profile.Tier = calculateTier(profile)

	// Check Ollama
	profile.OllamaStatus = checkOllamaStatus()

	return profile, nil
}

// detectMemory detects system RAM.
func detectMemory() int {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
			return int(bytes / (1024 * 1024 * 1024))
		}
	case "linux":
		out, err := exec.Command("grep", "MemTotal", "/proc/meminfo").Output()
		if err == nil {
			parts := strings.Fields(string(out))
			if len(parts) >= 2 {
				kb, _ := strconv.ParseInt(parts[1], 10, 64)
				return int(kb / (1024 * 1024))
			}
		}
	case "windows":
		out, err := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && line != "TotalPhysicalMemory" {
					bytes, _ := strconv.ParseInt(line, 10, 64)
					return int(bytes / (1024 * 1024 * 1024))
				}
			}
		}
	}
	return 8 // Default assumption
}

// detectGPU detects GPU information.
func detectGPU() GPUInfo {
	gpu := GPUInfo{Available: false, Type: "none"}

	switch runtime.GOOS {
	case "darwin":
		// Apple Silicon has unified memory, check for Metal support
		out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
		if err == nil {
			output := string(out)
			if strings.Contains(output, "Apple") {
				gpu.Available = true
				gpu.Type = "apple"
				gpu.Name = "Apple Silicon"
				// Apple Silicon shares RAM, estimate VRAM as half of RAM
				gpu.VRAM = detectMemory() / 2
			} else if strings.Contains(output, "AMD") {
				gpu.Available = true
				gpu.Type = "amd"
				gpu.Name = "AMD GPU"
			}
		}

	case "linux":
		// Check NVIDIA
		if out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), ",")
			if len(lines) >= 1 {
				gpu.Available = true
				gpu.Type = "nvidia"
				gpu.Name = strings.TrimSpace(lines[0])
				if len(lines) >= 2 {
					memStr := strings.TrimSpace(lines[1])
					memStr = strings.Replace(memStr, " MiB", "", 1)
					if mem, err := strconv.Atoi(memStr); err == nil {
						gpu.VRAM = mem / 1024
					}
				}
			}
		}
		// Check AMD
		if !gpu.Available {
			if _, err := os.Stat("/sys/class/drm/card0/device/vendor"); err == nil {
				out, _ := os.ReadFile("/sys/class/drm/card0/device/vendor")
				if strings.Contains(string(out), "0x1002") { // AMD vendor ID
					gpu.Available = true
					gpu.Type = "amd"
					gpu.Name = "AMD GPU"
				}
			}
		}

	case "windows":
		if out, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), ",")
			if len(lines) >= 1 {
				gpu.Available = true
				gpu.Type = "nvidia"
				gpu.Name = strings.TrimSpace(lines[0])
				if len(lines) >= 2 {
					memStr := strings.TrimSpace(lines[1])
					memStr = strings.Replace(memStr, " MiB", "", 1)
					if mem, err := strconv.Atoi(memStr); err == nil {
						gpu.VRAM = mem / 1024
					}
				}
			}
		}
	}

	return gpu
}

// calculateTier calculates the hardware tier.
func calculateTier(profile *SystemProfile) string {
	// Use VRAM if available, otherwise RAM
	effectiveMemory := profile.MemoryGB
	if profile.GPU.Available && profile.GPU.VRAM > 0 {
		effectiveMemory = profile.GPU.VRAM
	}

	switch {
	case effectiveMemory >= 48:
		return "ultra"
	case effectiveMemory >= 24:
		return "high"
	case effectiveMemory >= 12:
		return "mid"
	case effectiveMemory >= 6:
		return "low"
	default:
		return "minimal"
	}
}

// checkOllamaStatus checks Ollama installation and status.
func checkOllamaStatus() OllamaStatus {
	status := OllamaStatus{}

	// Check if Ollama is installed
	if _, err := exec.LookPath("ollama"); err == nil {
		status.Installed = true

		// Get version
		if out, err := exec.Command("ollama", "--version").Output(); err == nil {
			status.Version = strings.TrimSpace(string(out))
		}

		// Check if running by trying to list models
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if out, err := exec.CommandContext(ctx, "ollama", "list").Output(); err == nil {
			status.Running = true
			// Parse installed models
			lines := strings.Split(string(out), "\n")
			for _, line := range lines[1:] { // Skip header
				fields := strings.Fields(line)
				if len(fields) >= 1 {
					status.InstalledModels = append(status.InstalledModels, fields[0])
				}
			}
		} else {
			// Try to ping the API
			client := &http.Client{Timeout: 2 * time.Second}
			if resp, err := client.Get("http://localhost:11434/api/version"); err == nil {
				resp.Body.Close()
				status.Running = true
			}
		}
	}

	return status
}

// GetRecommendedModels returns models suitable for the system.
func GetRecommendedModels(profile *SystemProfile) []ModelSpec {
	var suitable []ModelSpec

	effectiveMemory := profile.MemoryGB
	if profile.GPU.Available && profile.GPU.VRAM > 0 {
		effectiveMemory = profile.GPU.VRAM
	}

	for _, model := range AvailableModels {
		if model.MinRAMGB <= profile.MemoryGB || model.MinVRAMGB <= effectiveMemory {
			suitable = append(suitable, model)
		}
	}

	// Sort by priority
	sort.Slice(suitable, func(i, j int) bool {
		return suitable[i].Priority < suitable[j].Priority
	})

	return suitable
}

// GetOptimalModelSet returns a balanced set of models for the system.
func GetOptimalModelSet(profile *SystemProfile) []ModelSpec {
	suitable := GetRecommendedModels(profile)
	if len(suitable) == 0 {
		return []ModelSpec{AvailableModels[len(AvailableModels)-1]} // Return smallest model
	}

	// Select one model from each category that fits
	selected := make(map[string]ModelSpec)
	categories := []string{"general", "code", "fast", "embedding"}

	for _, cat := range categories {
		for _, model := range suitable {
			if model.Category == cat {
				if _, exists := selected[cat]; !exists {
					selected[cat] = model
					break
				}
			}
		}
	}

	var result []ModelSpec
	for _, model := range selected {
		result = append(result, model)
	}

	// Sort by priority
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})

	return result
}

// CalculateTotalDownloadSize calculates total download size for models.
func CalculateTotalDownloadSize(models []ModelSpec) float64 {
	var total float64
	for _, m := range models {
		total += m.SizeGB
	}
	return total
}

// AutoSetupResult holds the result of auto setup.
type AutoSetupResult struct {
	Profile         *SystemProfile `json:"profile"`
	ModelsToInstall []ModelSpec    `json:"models_to_install"`
	TotalSizeGB     float64        `json:"total_size_gb"`
	OllamaInstalled bool           `json:"ollama_installed"`
	OllamaStarted   bool           `json:"ollama_started"`
	ModelsInstalled []string       `json:"models_installed"`
	Errors          []string       `json:"errors"`
}

// RunAutoSetup performs complete auto setup.
func RunAutoSetup(autoInstall bool) (*AutoSetupResult, error) {
	result := &AutoSetupResult{}

	// Detect system
	fmt.Println("🔍 Detecting system...")
	profile, err := DetectSystem()
	if err != nil {
		return nil, fmt.Errorf("failed to detect system: %w", err)
	}
	result.Profile = profile

	fmt.Printf("  ✓ OS: %s (%s)\n", profile.OS, profile.Arch)
	fmt.Printf("  ✓ Memory: %d GB\n", profile.MemoryGB)
	fmt.Printf("  ✓ CPU Cores: %d\n", profile.CPUCores)
	if profile.GPU.Available {
		fmt.Printf("  ✓ GPU: %s (%s) - %d GB VRAM\n", profile.GPU.Name, profile.GPU.Type, profile.GPU.VRAM)
	} else {
		fmt.Println("  ✓ GPU: None detected")
	}
	fmt.Printf("  ✓ Tier: %s\n", profile.Tier)
	fmt.Println()

	// Check/Install Ollama
	fmt.Println("🔧 Checking Ollama...")
	if !profile.OllamaStatus.Installed {
		fmt.Println("  ⚠ Ollama not installed")
		if autoInstall {
			fmt.Println("  📦 Installing Ollama...")
			if err := InstallOllama(); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to install Ollama: %v", err))
				fmt.Printf("  ✗ Failed: %v\n", err)
				fmt.Println("  Please install manually: https://ollama.ai/download")
			} else {
				fmt.Println("  ✓ Ollama installed")
				result.OllamaInstalled = true
				// Re-check status
				profile.OllamaStatus = checkOllamaStatus()
			}
		} else {
			fmt.Println("  Run with DEVORCH_AUTO_INSTALL=1 to auto-install")
		}
	} else {
		fmt.Printf("  ✓ Ollama installed (v%s)\n", profile.OllamaStatus.Version)
		result.OllamaInstalled = true
	}
	fmt.Println()

	// Start Ollama if not running
	if result.OllamaInstalled && !profile.OllamaStatus.Running {
		fmt.Println("🚀 Starting Ollama...")
		if err := StartOllama(); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to start Ollama: %v", err))
			fmt.Printf("  ✗ Failed: %v\n", err)
		} else {
			fmt.Println("  ✓ Ollama started")
			result.OllamaStarted = true
		}
		fmt.Println()
	} else if profile.OllamaStatus.Running {
		result.OllamaStarted = true
		fmt.Println("  ✓ Ollama is running")
	}

	// Get recommended models
	fmt.Println("📋 Selecting optimal models...")
	models := GetOptimalModelSet(profile)
	result.ModelsToInstall = models
	result.TotalSizeGB = CalculateTotalDownloadSize(models)

	fmt.Printf("  Selected %d models (%.1f GB total):\n", len(models), result.TotalSizeGB)
	for _, m := range models {
		fullName := m.Name + ":" + m.Tag
		installed := ""
		for _, im := range profile.OllamaStatus.InstalledModels {
			if strings.HasPrefix(im, m.Name) {
				installed = " [installed]"
				break
			}
		}
		fmt.Printf("    • %s (%.1f GB) - %s%s\n", fullName, m.SizeGB, m.Description, installed)
	}
	fmt.Println()

	// Install models if requested
	if autoInstall && result.OllamaStarted {
		fmt.Println("⬇️ Installing models...")
		for _, m := range models {
			fullName := m.Name + ":" + m.Tag
			// Check if already installed
			alreadyInstalled := false
			for _, im := range profile.OllamaStatus.InstalledModels {
				if strings.HasPrefix(im, m.Name) {
					alreadyInstalled = true
					break
				}
			}
			if alreadyInstalled {
				fmt.Printf("  ✓ %s already installed\n", fullName)
				result.ModelsInstalled = append(result.ModelsInstalled, fullName)
				continue
			}

			fmt.Printf("  ⬇ Pulling %s...\n", fullName)
			if err := PullModel(fullName); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to pull %s: %v", fullName, err))
				fmt.Printf("  ✗ Failed: %v\n", err)
			} else {
				fmt.Printf("  ✓ %s installed\n", fullName)
				result.ModelsInstalled = append(result.ModelsInstalled, fullName)
			}
		}
		fmt.Println()
	}

	return result, nil
}

// InstallOllama installs Ollama based on OS.
func InstallOllama() error {
	switch runtime.GOOS {
	case "darwin":
		// Try Homebrew first
		if _, err := exec.LookPath("brew"); err == nil {
			cmd := exec.Command("brew", "install", "ollama")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
		// Fallback to curl installer
		cmd := exec.Command("sh", "-c", "curl -fsSL https://ollama.ai/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "linux":
		cmd := exec.Command("sh", "-c", "curl -fsSL https://ollama.ai/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "windows":
		return fmt.Errorf("please install Ollama manually from https://ollama.ai/download")

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// StartOllama starts the Ollama service.
func StartOllama() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// Try brew services first
		if _, err := exec.LookPath("brew"); err == nil {
			cmd = exec.Command("brew", "services", "start", "ollama")
			if err := cmd.Run(); err == nil {
				time.Sleep(2 * time.Second)
				return nil
			}
		}
		// Background start
		cmd = exec.Command("ollama", "serve")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			return err
		}

	case "linux":
		// Try systemd
		cmd = exec.Command("systemctl", "--user", "start", "ollama")
		if err := cmd.Run(); err == nil {
			time.Sleep(2 * time.Second)
			return nil
		}
		// Background start
		cmd = exec.Command("ollama", "serve")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			return err
		}

	case "windows":
		cmd = exec.Command("cmd", "/C", "start", "/b", "ollama", "serve")
		if err := cmd.Start(); err != nil {
			return err
		}
	}

	// Wait for Ollama to be ready
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if resp, err := client.Get("http://localhost:11434/api/version"); err == nil {
			resp.Body.Close()
			return nil
		}
	}

	return nil // May still be starting
}

// PullModel pulls a model using Ollama.
func PullModel(model string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ollama", "pull", model)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// PrintSystemInfo prints system info in a formatted way.
func PrintSystemInfo(profile *SystemProfile) {
	data, _ := json.MarshalIndent(profile, "", "  ")
	fmt.Println(string(data))
}
