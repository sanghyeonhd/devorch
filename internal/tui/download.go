// Package tui provides download progress tracking for Ollama models
package tui

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DownloadProgress tracks model download progress
type DownloadProgress struct {
	Model      string
	Status     string // "pulling", "downloading", "verifying", "done", "error"
	TotalBytes int64
	Downloaded int64
	Speed      float64 // bytes per second
	StartTime  time.Time
	LastUpdate time.Time
	ETA        time.Duration
	Percent    float64
	Layer      string // Current layer being downloaded
	LayerIndex int
	LayerTotal int
	Error      error
	mu         sync.RWMutex
}

// NewDownloadProgress creates a new download progress tracker
func NewDownloadProgress(model string) *DownloadProgress {
	return &DownloadProgress{
		Model:     model,
		Status:    "starting",
		StartTime: time.Now(),
	}
}

// Update updates progress from an Ollama output line
func (d *DownloadProgress) Update(line string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.LastUpdate = time.Now()

	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Parse "pulling manifest"
	if strings.Contains(line, "pulling manifest") {
		d.Status = "pulling"
		return
	}

	// Parse "pulling <hash>" with layer info
	// Format: "pulling abc123... 100% ▕████████████████▏ 1.2 GB"
	pullPattern := regexp.MustCompile(`pulling\s+([a-f0-9]+)`)
	if matches := pullPattern.FindStringSubmatch(line); len(matches) > 1 {
		d.Layer = matches[1]
		d.Status = "downloading"
	}

	// Parse percentage
	// Format: "45% ▕████░░░░░░░░░░▏" or just "45%"
	percentPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)
	if matches := percentPattern.FindStringSubmatch(line); len(matches) > 1 {
		if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
			d.Percent = p
		}
	}

	// Parse size info
	// Format: "1.2 GB/2.7 GB" or "1.2 GB"
	sizePattern := regexp.MustCompile(`([\d.]+)\s*(GB|MB|KB|B)\s*/\s*([\d.]+)\s*(GB|MB|KB|B)`)
	if matches := sizePattern.FindStringSubmatch(line); len(matches) > 4 {
		downloaded := parseSize(matches[1], matches[2])
		total := parseSize(matches[3], matches[4])
		d.Downloaded = downloaded
		d.TotalBytes = total
	}

	// Parse speed
	// Format: "15.2 MB/s" or "1.2 GB/s"
	speedPattern := regexp.MustCompile(`([\d.]+)\s*(GB|MB|KB|B)/s`)
	if matches := speedPattern.FindStringSubmatch(line); len(matches) > 2 {
		d.Speed = float64(parseSize(matches[1], matches[2]))
	}

	// Calculate ETA
	if d.Speed > 0 && d.TotalBytes > 0 && d.Downloaded > 0 {
		remaining := d.TotalBytes - d.Downloaded
		d.ETA = time.Duration(float64(remaining)/d.Speed) * time.Second
	}

	// Check for completion states
	if strings.Contains(line, "verifying") {
		d.Status = "verifying"
		d.Percent = 100
	} else if strings.Contains(line, "writing") {
		d.Status = "writing"
	} else if strings.Contains(line, "success") {
		d.Status = "done"
		d.Percent = 100
	}
}

// parseSize converts size string to bytes
func parseSize(value, unit string) int64 {
	v, _ := strconv.ParseFloat(value, 64)
	switch strings.ToUpper(unit) {
	case "GB":
		return int64(v * 1024 * 1024 * 1024)
	case "MB":
		return int64(v * 1024 * 1024)
	case "KB":
		return int64(v * 1024)
	default:
		return int64(v)
	}
}

// formatBytes formats bytes to human readable string
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Render returns the progress bar string
func (d *DownloadProgress) Render(width int) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	switch d.Status {
	case "starting":
		return fmt.Sprintf("  ⏳ Preparing to download %s...", d.Model)
	case "pulling":
		return fmt.Sprintf("  📥 Pulling manifest for %s...", d.Model)
	case "verifying":
		return fmt.Sprintf("  ✅ Verifying %s...", d.Model)
	case "writing":
		return fmt.Sprintf("  💾 Writing %s...", d.Model)
	case "done":
		elapsed := time.Since(d.StartTime).Round(time.Second)
		return fmt.Sprintf("  ✓ %s installed successfully (took %s)", d.Model, elapsed)
	case "error":
		return fmt.Sprintf("  ✗ Error: %v", d.Error)
	}

	// Downloading state - show progress bar
	barWidth := width - 50 // Leave space for stats
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(d.Percent / 100 * float64(barWidth))
	empty := barWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	// Format speed
	speedStr := ""
	if d.Speed > 0 {
		speedStr = fmt.Sprintf(" %s/s", formatBytes(int64(d.Speed)))
	}

	// Format ETA
	etaStr := ""
	if d.ETA > 0 && d.ETA < 24*time.Hour {
		etaStr = fmt.Sprintf(" ETA: %s", d.ETA.Round(time.Second))
	}

	// Format size
	sizeStr := ""
	if d.TotalBytes > 0 {
		sizeStr = fmt.Sprintf(" %s/%s", formatBytes(d.Downloaded), formatBytes(d.TotalBytes))
	}

	return fmt.Sprintf("  ⬇ %s\n  %s %.1f%%%s%s%s",
		d.Model, bar, d.Percent, sizeStr, speedStr, etaStr)
}

// IsComplete returns true if download is complete or errored
func (d *DownloadProgress) IsComplete() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Status == "done" || d.Status == "error"
}

// InstallModelWithProgress installs an Ollama model with progress tracking
func InstallModelWithProgress(model string, theme *Theme) error {
	progress := NewDownloadProgress(model)

	fmt.Println()
	fmt.Println(theme.Title.Render("  📦 Installing Model  "))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	cmd := exec.Command("ollama", "pull", model)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	// Read stdout and stderr concurrently
	var wg sync.WaitGroup

	// Read stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		lastRender := time.Now()
		for scanner.Scan() {
			line := scanner.Text()
			progress.Update(line)

			// Rate limit rendering to avoid flicker
			if time.Since(lastRender) > 100*time.Millisecond {
				// Move cursor up and clear line, then print new progress
				fmt.Print("\033[2K\r") // Clear current line
				fmt.Print(progress.Render(60))
				lastRender = time.Now()
			}
		}
	}()

	// Read stderr (Ollama might output progress here too)
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			progress.Update(line)
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		progress.mu.Lock()
		progress.Status = "error"
		progress.Error = err
		progress.mu.Unlock()
		fmt.Println()
		fmt.Println(theme.Error.Render(progress.Render(60)))
		return err
	}

	progress.mu.Lock()
	progress.Status = "done"
	progress.mu.Unlock()

	fmt.Println()
	fmt.Println(theme.Success.Render(progress.Render(60)))
	fmt.Println()
	fmt.Printf("  Use %s to switch to this model\n", theme.Accent.Render("/model "+model))
	fmt.Println()

	return nil
}
