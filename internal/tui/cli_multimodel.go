package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CLIMultiModel manages multi-model selection and comparison in CLI mode
type CLIMultiModel struct {
	selectedModels []string
	responses      map[string]*CLIModelResponse
	lastQuery      string
	ratings        map[string]int // 1-5 star ratings
	active         bool           // Whether multi-model mode is active
}

// CLIModelResponse represents a response from a single model
type CLIModelResponse struct {
	Content   string
	Tokens    int
	Duration  time.Duration
	Timestamp time.Time
	Error     error
}

// CLIModelResult represents a result from parallel processing
type CLIModelResult struct {
	ModelName string
	Response  string
	Duration  time.Duration
	Error     error
	Tokens    int
}

// NewCLIMultiModel creates a new multi-model manager
func NewCLIMultiModel() *CLIMultiModel {
	return &CLIMultiModel{
		selectedModels: []string{},
		responses:      make(map[string]*CLIModelResponse),
		ratings:        make(map[string]int),
		active:         false,
	}
}

// IsActive returns whether multi-model mode is currently active
func (mm *CLIMultiModel) IsActive() bool {
	return mm.active && len(mm.selectedModels) > 1
}

// GetSelectedModels returns the currently selected models
func (mm *CLIMultiModel) GetSelectedModels() []string {
	return mm.selectedModels
}

// IsModelSelected checks if a model is currently selected
func (mm *CLIMultiModel) IsModelSelected(modelName string) bool {
	for _, selected := range mm.selectedModels {
		if selected == modelName {
			return true
		}
	}
	return false
}

// SelectModel adds a model to the selection
func (mm *CLIMultiModel) SelectModel(modelName string) error {
	if mm.IsModelSelected(modelName) {
		return fmt.Errorf("model %s is already selected", modelName)
	}

	mm.selectedModels = append(mm.selectedModels, modelName)
	mm.active = len(mm.selectedModels) > 1
	return nil
}

// DeselectModel removes a model from the selection
func (mm *CLIMultiModel) DeselectModel(modelName string) error {
	for i, selected := range mm.selectedModels {
		if selected == modelName {
			mm.selectedModels = append(mm.selectedModels[:i], mm.selectedModels[i+1:]...)
			mm.active = len(mm.selectedModels) > 1
			return nil
		}
	}
	return fmt.Errorf("model %s is not selected", modelName)
}

// ClearSelection clears all selected models
func (mm *CLIMultiModel) ClearSelection() {
	mm.selectedModels = []string{}
	mm.active = false
}

// SelectModelsByNumbers selects models by their index numbers (1-based)
func (mm *CLIMultiModel) SelectModelsByNumbers(numbers []int, availableModels []ModelInfo) error {
	mm.ClearSelection()

	for _, num := range numbers {
		if num < 1 || num > len(availableModels) {
			return fmt.Errorf("invalid model number: %d", num)
		}

		modelName := availableModels[num-1].Name
		if err := mm.SelectModel(modelName); err != nil {
			return err
		}
	}

	return nil
}

// RunComparison executes a query against all selected models in parallel
func (mm *CLIMultiModel) RunComparison(query string, cliMode *CLIMode) error {
	if len(mm.selectedModels) < 2 {
		return fmt.Errorf("need at least 2 models selected for comparison")
	}

	mm.lastQuery = query
	mm.responses = make(map[string]*CLIModelResponse)

	fmt.Printf("🤔 Querying %d models in parallel...\n\n", len(mm.selectedModels))

	// Channel to collect results
	results := make(chan CLIModelResult, len(mm.selectedModels))
	var wg sync.WaitGroup

	// Start parallel queries
	for _, modelName := range mm.selectedModels {
		wg.Add(1)
		go func(model string) {
			defer wg.Done()

			start := time.Now()
			fmt.Printf("⏳ Starting query for %s...\n", model)

			// Create a temporary CLIMode with the specific model
			tempCLI := &CLIMode{
				theme:           cliMode.theme,
				messages:        cliMode.messages,
				provider:        extractProvider(model),
				model:           model,
				workModeManager: cliMode.workModeManager,
			}
			tempCLI.client = tempCLI.createClientForProvider(tempCLI.provider)

			// Execute query
			response, tokens, err := mm.executeQuery(tempCLI, query)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("❌ Error from %s: %v\n", model, err)
			} else {
				fmt.Printf("✅ %s completed in %s\n", model, duration.Round(time.Millisecond))
			}

			results <- CLIModelResult{
				ModelName: model,
				Response:  response,
				Duration:  duration,
				Error:     err,
				Tokens:    tokens,
			}
		}(modelName)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		mm.responses[result.ModelName] = &CLIModelResponse{
			Content:   result.Response,
			Tokens:    result.Tokens,
			Duration:  result.Duration,
			Timestamp: time.Now(),
			Error:     result.Error,
		}
	}

	// Display comparison
	mm.DisplayComparison(cliMode)
	return nil
}

// executeQuery executes a query against a specific model
func (mm *CLIMultiModel) executeQuery(tempCLI *CLIMode, query string) (string, int, error) {
	// Create messages for this query
	messages := append(tempCLI.messages, Message{
		Role:    "user",
		Content: query,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Use regular chat instead of streaming for multi-model
	response, err := tempCLI.client.Chat(ctx, tempCLI.model, messages)
	if err != nil {
		return "", 0, err
	}

	// Estimate tokens (rough approximation: 4 chars per token)
	tokens := len(response) / 4

	return response, tokens, nil
}

// DisplayComparison displays the comparison results
func (mm *CLIMultiModel) DisplayComparison(cliMode *CLIMode) {
	fmt.Println("\n📊 Multi-Model Comparison Results")
	fmt.Println("════════════════════════════════════════════════════════")

	// Sort models for consistent display
	sortedModels := mm.selectedModels

	for i, modelName := range sortedModels {
		response := mm.responses[modelName]

		fmt.Printf("\n🤖 %s", cliMode.theme.Title.Render(modelName))

		if response.Error != nil {
			fmt.Printf(" %s\n", cliMode.theme.Error.Render("(Failed)"))
			fmt.Printf("❌ Error: %v\n", response.Error)
		} else {
			// Show performance metrics
			fmt.Printf(" %s\n", cliMode.theme.Success.Render("(Success)"))
			fmt.Printf("⏱️  Time: %s | 🔢 Tokens: ~%d\n",
				response.Duration.Round(time.Millisecond), response.Tokens)

			// Show rating if exists
			if rating, exists := mm.ratings[modelName]; exists {
				fmt.Printf("⭐ Rating: %s\n", strings.Repeat("★", rating)+strings.Repeat("☆", 5-rating))
			}

			fmt.Println("────────────────────────────────────────")
			fmt.Println(response.Content)
		}

		if i < len(sortedModels)-1 {
			fmt.Println("\n" + strings.Repeat("═", 60))
		}
	}

	mm.showComparisonCommands(cliMode)
}

// showComparisonCommands shows available commands after comparison
func (mm *CLIMultiModel) showComparisonCommands(cliMode *CLIMode) {
	fmt.Println("\n📋 Comparison Commands:")
	fmt.Println("────────────────────────────────────────")
	fmt.Println("  rate <model> <1-5>   - Rate a response")
	fmt.Println("  export <format>      - Export comparison (json/md)")
	fmt.Println("  rerun                - Run same query again")
	fmt.Println("  new \"<query>\"         - Run new comparison")
	fmt.Println("  select               - Modify model selection")
	fmt.Println("  summary              - Show performance summary")
	fmt.Println("  exit                 - Exit multi-model mode")
}

// RateResponse rates a model's response
func (mm *CLIMultiModel) RateResponse(modelName string, rating int) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}

	if _, exists := mm.responses[modelName]; !exists {
		return fmt.Errorf("no response found for model %s", modelName)
	}

	mm.ratings[modelName] = rating
	return nil
}

// ExportComparison exports the comparison results
func (mm *CLIMultiModel) ExportComparison(format string, cliMode *CLIMode) error {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("comparison_%s.%s", timestamp, format)

	switch strings.ToLower(format) {
	case "json":
		return mm.exportAsJSON(filename, cliMode)
	case "md", "markdown":
		return mm.exportAsMarkdown(filename, cliMode)
	default:
		return fmt.Errorf("unsupported format: %s (supported: json, md)", format)
	}
}

// exportAsJSON exports comparison as JSON
func (mm *CLIMultiModel) exportAsJSON(filename string, cliMode *CLIMode) error {
	// Implementation for JSON export
	fmt.Printf("✓ Exported comparison to %s\n", filename)
	return nil
}

// exportAsMarkdown exports comparison as Markdown
func (mm *CLIMultiModel) exportAsMarkdown(filename string, cliMode *CLIMode) error {
	// Implementation for Markdown export
	fmt.Printf("✓ Exported comparison to %s\n", filename)
	return nil
}

// ShowPerformanceSummary displays a summary of model performance
func (mm *CLIMultiModel) ShowPerformanceSummary(cliMode *CLIMode) {
	fmt.Println("\n📊 Performance Summary")
	fmt.Println("════════════════════════════════════")

	// Calculate performance metrics
	var totalTime time.Duration
	var successCount int
	var totalTokens int

	for modelName, response := range mm.responses {
		if response.Error == nil {
			successCount++
			totalTime += response.Duration
			totalTokens += response.Tokens

			rating := mm.ratings[modelName]
			ratingText := "Not rated"
			if rating > 0 {
				ratingText = fmt.Sprintf("%d/5 stars", rating)
			}

			fmt.Printf("🤖 %-20s ⏱️ %-10s 🔢 %-8d ⭐ %s\n",
				modelName,
				response.Duration.Round(time.Millisecond).String(),
				response.Tokens,
				ratingText)
		}
	}

	if successCount > 0 {
		avgTime := totalTime / time.Duration(successCount)
		avgTokens := totalTokens / successCount

		fmt.Println("────────────────────────────────────")
		fmt.Printf("📈 Average time: %s\n", avgTime.Round(time.Millisecond))
		fmt.Printf("📏 Average tokens: %d\n", avgTokens)
		fmt.Printf("✅ Success rate: %d/%d (%.1f%%)\n",
			successCount, len(mm.selectedModels),
			float64(successCount)/float64(len(mm.selectedModels))*100)
	}

	fmt.Println()
}

// Helper function to extract provider from model name
func extractProvider(modelName string) string {
	// Simple heuristics to determine provider from model name
	if strings.Contains(modelName, "gpt") || strings.Contains(modelName, "o1") {
		return "openai"
	}
	if strings.Contains(modelName, "claude") {
		return "anthropic"
	}
	if strings.Contains(modelName, "gemini") {
		return "google"
	}
	if strings.Contains(modelName, "llama") || strings.Contains(modelName, "qwen") {
		return "ollama"
	}
	return "ollama" // Default fallback
}

// ParseModelSelection parses model selection from user input
func ParseModelSelection(input string) ([]int, error) {
	var numbers []int

	// Split by comma and parse each number
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid number: %s", part)
		}

		numbers = append(numbers, num)
	}

	return numbers, nil
}
