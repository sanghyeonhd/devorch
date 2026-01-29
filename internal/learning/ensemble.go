// Package learning implements ensemble execution strategies for multi-LLM orchestration
package learning

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// LLMClient is the interface for calling LLM models
type LLMClient interface {
	Call(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
	Provider() string
	Model() string
}

// LLMRequest is the input for an LLM call
type LLMRequest struct {
	Messages    []Message      `json:"messages"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	TaskType    string         `json:"task_type,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse is the output from an LLM call
type LLMResponse struct {
	Content      string         `json:"content"`
	TokensUsed   int64          `json:"tokens_used"`
	LatencyMs    int64          `json:"latency_ms"`
	CostMicro    int64          `json:"cost_micro"`
	Provider     string         `json:"provider"`
	Model        string         `json:"model"`
	FinishReason string         `json:"finish_reason,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// EnsembleExecutor executes multi-model ensemble strategies
type EnsembleExecutor struct {
	orchestrator *MultiLLMOrchestrator
	clients      map[string]LLMClient // key: provider:model
	timeout      time.Duration
}

// NewEnsembleExecutor creates a new ensemble executor
func NewEnsembleExecutor(orchestrator *MultiLLMOrchestrator, clients map[string]LLMClient) *EnsembleExecutor {
	return &EnsembleExecutor{
		orchestrator: orchestrator,
		clients:      clients,
		timeout:      60 * time.Second,
	}
}

// AddClient adds an LLM client
func (e *EnsembleExecutor) AddClient(key string, client LLMClient) {
	if e.clients == nil {
		e.clients = make(map[string]LLMClient)
	}
	e.clients[key] = client
}

// ExecuteRequest executes a request using the ensemble strategy
func (e *EnsembleExecutor) ExecuteRequest(ctx context.Context, req *LLMRequest) (*EnsembleResult, error) {
	// Get available candidates
	candidates := make([]string, 0, len(e.clients))
	for key := range e.clients {
		candidates = append(candidates, key)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no LLM clients available")
	}

	// Let orchestrator select models and strategy
	selReq := SelectionRequest{
		TaskType:   req.TaskType,
		Complexity: e.estimateComplexity(req),
		Candidates: candidates,
	}

	selection, err := e.orchestrator.SelectModels(ctx, selReq)
	if err != nil {
		return nil, fmt.Errorf("model selection failed: %w", err)
	}

	// Execute based on strategy
	var result *EnsembleResult
	switch selection.Strategy {
	case StrategyBestSingle:
		result, err = e.executeSingle(ctx, req, selection)
	case StrategyParallel:
		result, err = e.executeParallel(ctx, req, selection)
	case StrategyChain:
		result, err = e.executeChain(ctx, req, selection)
	case StrategyVoting:
		result, err = e.executeVoting(ctx, req, selection)
	case StrategySpecialized:
		result, err = e.executeSingle(ctx, req, selection) // Same as single
	case StrategyAdaptive:
		// Should not reach here as orchestrator resolves adaptive
		result, err = e.executeParallel(ctx, req, selection)
	default:
		result, err = e.executeSingle(ctx, req, selection)
	}

	if err != nil {
		return nil, err
	}

	result.Strategy = selection.Strategy
	result.Explanation = selection.Explanation
	return result, nil
}

// EnsembleResult is the result of ensemble execution
type EnsembleResult struct {
	FinalResponse  *LLMResponse     `json:"final_response"`
	AllResponses   []*LLMResponse   `json:"all_responses,omitempty"`
	Strategy       EnsembleStrategy `json:"strategy"`
	Confidence     float64          `json:"confidence"`
	Explanation    string           `json:"explanation"`
	TotalLatencyMs int64            `json:"total_latency_ms"`
	TotalCostMicro int64            `json:"total_cost_micro"`
	ModelsUsed     []string         `json:"models_used"`
}

// executeSingle executes with a single model
func (e *EnsembleExecutor) executeSingle(ctx context.Context, req *LLMRequest, sel *SelectionResult) (*EnsembleResult, error) {
	if len(sel.SelectedModels) == 0 {
		return nil, fmt.Errorf("no models selected")
	}

	modelKey := sel.SelectedModels[0]
	client, ok := e.clients[modelKey]
	if !ok {
		return nil, fmt.Errorf("client not found for %s", modelKey)
	}

	start := time.Now()
	resp, err := client.Call(ctx, req)
	latency := time.Since(start).Milliseconds()

	// Record outcome for learning
	e.recordOutcome(ctx, client.Provider(), client.Model(), req.TaskType, resp, err, latency)

	if err != nil {
		return nil, err
	}

	resp.LatencyMs = latency

	return &EnsembleResult{
		FinalResponse:  resp,
		AllResponses:   []*LLMResponse{resp},
		Confidence:     sel.Confidence,
		TotalLatencyMs: latency,
		TotalCostMicro: resp.CostMicro,
		ModelsUsed:     []string{modelKey},
	}, nil
}

// executeParallel executes models in parallel and picks the best response
func (e *EnsembleExecutor) executeParallel(ctx context.Context, req *LLMRequest, sel *SelectionResult) (*EnsembleResult, error) {
	if len(sel.SelectedModels) == 0 {
		return nil, fmt.Errorf("no models selected")
	}

	// Create cancellable context
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	type callResult struct {
		key      string
		response *LLMResponse
		err      error
		latency  int64
	}

	results := make(chan callResult, len(sel.SelectedModels))
	var wg sync.WaitGroup

	start := time.Now()

	// Launch parallel calls
	for _, modelKey := range sel.SelectedModels {
		client, ok := e.clients[modelKey]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(key string, c LLMClient) {
			defer wg.Done()

			callStart := time.Now()
			resp, err := c.Call(ctx, req)
			latency := time.Since(callStart).Milliseconds()

			if resp != nil {
				resp.LatencyMs = latency
			}

			results <- callResult{
				key:      key,
				response: resp,
				err:      err,
				latency:  latency,
			}
		}(modelKey, client)
	}

	// Close results channel when all done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResponses []*LLMResponse
	var modelsUsed []string
	var totalCost int64

	for res := range results {
		if res.err == nil && res.response != nil {
			allResponses = append(allResponses, res.response)
			modelsUsed = append(modelsUsed, res.key)
			totalCost += res.response.CostMicro

			// Record outcome for learning
			parts := splitKey(res.key)
			e.recordOutcome(ctx, parts[0], parts[1], req.TaskType, res.response, nil, res.latency)
		} else if res.err != nil {
			// Record failure
			parts := splitKey(res.key)
			e.recordOutcome(ctx, parts[0], parts[1], req.TaskType, nil, res.err, res.latency)
		}
	}

	totalLatency := time.Since(start).Milliseconds()

	if len(allResponses) == 0 {
		return nil, fmt.Errorf("all parallel calls failed")
	}

	// Select best response (by quality estimate based on length and content)
	best := e.selectBestResponse(allResponses)

	return &EnsembleResult{
		FinalResponse:  best,
		AllResponses:   allResponses,
		Confidence:     sel.Confidence * float64(len(allResponses)) / float64(len(sel.SelectedModels)),
		TotalLatencyMs: totalLatency,
		TotalCostMicro: totalCost,
		ModelsUsed:     modelsUsed,
	}, nil
}

// executeChain executes models in a chain (draft -> refine)
func (e *EnsembleExecutor) executeChain(ctx context.Context, req *LLMRequest, sel *SelectionResult) (*EnsembleResult, error) {
	if len(sel.SelectedModels) < 2 {
		// Fall back to single
		return e.executeSingle(ctx, req, sel)
	}

	var allResponses []*LLMResponse
	var modelsUsed []string
	var totalCost int64
	start := time.Now()

	// First model: Draft
	draftKey := sel.SelectedModels[0]
	draftClient, ok := e.clients[draftKey]
	if !ok {
		return nil, fmt.Errorf("draft client not found: %s", draftKey)
	}

	draftStart := time.Now()
	draftResp, err := draftClient.Call(ctx, req)
	draftLatency := time.Since(draftStart).Milliseconds()

	e.recordOutcome(ctx, draftClient.Provider(), draftClient.Model(), req.TaskType, draftResp, err, draftLatency)

	if err != nil {
		return nil, fmt.Errorf("draft failed: %w", err)
	}

	draftResp.LatencyMs = draftLatency
	allResponses = append(allResponses, draftResp)
	modelsUsed = append(modelsUsed, draftKey)
	totalCost += draftResp.CostMicro

	// Second model: Refine
	refineKey := sel.SelectedModels[1]
	refineClient, ok := e.clients[refineKey]
	if !ok {
		// Return draft if refiner not available
		return &EnsembleResult{
			FinalResponse:  draftResp,
			AllResponses:   allResponses,
			Confidence:     sel.Confidence * 0.8,
			TotalLatencyMs: time.Since(start).Milliseconds(),
			TotalCostMicro: totalCost,
			ModelsUsed:     modelsUsed,
		}, nil
	}

	// Build refinement request
	refineReq := &LLMRequest{
		Messages: []Message{
			{Role: "system", Content: "You are a code reviewer and refiner. Improve the following response by fixing any issues, improving clarity, and ensuring correctness."},
			{Role: "user", Content: fmt.Sprintf("Original request: %s\n\nDraft response:\n%s\n\nPlease review and improve this response.",
				req.Messages[len(req.Messages)-1].Content, draftResp.Content)},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: 0.3, // Lower temperature for refinement
		TaskType:    req.TaskType,
	}

	refineStart := time.Now()
	refineResp, err := refineClient.Call(ctx, refineReq)
	refineLatency := time.Since(refineStart).Milliseconds()

	e.recordOutcome(ctx, refineClient.Provider(), refineClient.Model(), req.TaskType, refineResp, err, refineLatency)

	if err != nil {
		// Return draft if refinement fails
		return &EnsembleResult{
			FinalResponse:  draftResp,
			AllResponses:   allResponses,
			Confidence:     sel.Confidence * 0.7,
			TotalLatencyMs: time.Since(start).Milliseconds(),
			TotalCostMicro: totalCost,
			ModelsUsed:     modelsUsed,
		}, nil
	}

	refineResp.LatencyMs = refineLatency
	allResponses = append(allResponses, refineResp)
	modelsUsed = append(modelsUsed, refineKey)
	totalCost += refineResp.CostMicro

	return &EnsembleResult{
		FinalResponse:  refineResp,
		AllResponses:   allResponses,
		Confidence:     sel.Confidence,
		TotalLatencyMs: time.Since(start).Milliseconds(),
		TotalCostMicro: totalCost,
		ModelsUsed:     modelsUsed,
	}, nil
}

// executeVoting executes models and uses voting for consensus
func (e *EnsembleExecutor) executeVoting(ctx context.Context, req *LLMRequest, sel *SelectionResult) (*EnsembleResult, error) {
	// Get parallel responses first
	result, err := e.executeParallel(ctx, req, sel)
	if err != nil {
		return nil, err
	}

	if len(result.AllResponses) < 2 {
		return result, nil
	}

	// Use a voting model to determine consensus (or simple heuristic)
	// For now, use the most similar response to others as the consensus
	result.FinalResponse = e.findConsensusResponse(result.AllResponses)
	result.Confidence *= 0.9 // Slight confidence reduction for voting uncertainty

	return result, nil
}

// selectBestResponse selects the best response from multiple options
func (e *EnsembleExecutor) selectBestResponse(responses []*LLMResponse) *LLMResponse {
	if len(responses) == 0 {
		return nil
	}
	if len(responses) == 1 {
		return responses[0]
	}

	// Score each response
	type scoredResponse struct {
		resp  *LLMResponse
		score float64
	}

	scored := make([]scoredResponse, len(responses))
	for i, resp := range responses {
		score := e.scoreResponse(resp)
		scored[i] = scoredResponse{resp: resp, score: score}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return scored[0].resp
}

// scoreResponse calculates a quality score for a response
func (e *EnsembleExecutor) scoreResponse(resp *LLMResponse) float64 {
	if resp == nil {
		return 0
	}

	score := 0.5 // Base score

	// Length score (prefer reasonable length)
	contentLen := len(resp.Content)
	if contentLen > 100 && contentLen < 10000 {
		score += 0.2
	} else if contentLen >= 50 {
		score += 0.1
	}

	// Latency score (faster is better)
	if resp.LatencyMs < 2000 {
		score += 0.2
	} else if resp.LatencyMs < 5000 {
		score += 0.1
	}

	// Finish reason score
	if resp.FinishReason == "stop" || resp.FinishReason == "end_turn" {
		score += 0.1
	}

	return score
}

// findConsensusResponse finds the response most similar to others (voting consensus)
func (e *EnsembleExecutor) findConsensusResponse(responses []*LLMResponse) *LLMResponse {
	if len(responses) == 0 {
		return nil
	}
	if len(responses) == 1 {
		return responses[0]
	}

	// Calculate similarity scores (using simple length-based similarity)
	// In production, would use semantic similarity
	avgLen := 0
	for _, r := range responses {
		avgLen += len(r.Content)
	}
	avgLen /= len(responses)

	// Find response closest to average length
	var best *LLMResponse
	bestDiff := int(^uint(0) >> 1) // Max int

	for _, r := range responses {
		diff := abs(len(r.Content) - avgLen)
		if diff < bestDiff {
			bestDiff = diff
			best = r
		}
	}

	return best
}

// recordOutcome records the result for learning
func (e *EnsembleExecutor) recordOutcome(ctx context.Context, provider, model, taskType string, resp *LLMResponse, err error, latency int64) {
	if e.orchestrator == nil {
		return
	}

	outcome := &ModelOutcome{
		Provider:  provider,
		Model:     model,
		TaskType:  taskType,
		Success:   err == nil && resp != nil,
		LatencyMs: latency,
	}

	if resp != nil {
		outcome.Tokens = resp.TokensUsed
		outcome.CostMicro = resp.CostMicro
		outcome.Quality = e.estimateQuality(resp)
	} else {
		outcome.Quality = 0
	}

	e.orchestrator.RecordOutcome(ctx, outcome)
}

// estimateQuality estimates response quality
func (e *EnsembleExecutor) estimateQuality(resp *LLMResponse) float64 {
	if resp == nil {
		return 0
	}

	quality := 0.5

	// Content length (prefer substantive responses)
	contentLen := len(resp.Content)
	if contentLen > 200 {
		quality += 0.2
	} else if contentLen > 50 {
		quality += 0.1
	}

	// Proper finish
	if resp.FinishReason == "stop" || resp.FinishReason == "end_turn" {
		quality += 0.2
	}

	// Latency (reasonable response time indicates good quality)
	if resp.LatencyMs > 0 && resp.LatencyMs < 3000 {
		quality += 0.1
	}

	return clamp01(quality)
}

// estimateComplexity estimates request complexity (0-1)
func (e *EnsembleExecutor) estimateComplexity(req *LLMRequest) float64 {
	if req == nil {
		return 0.5
	}

	complexity := 0.3 // Base

	// Message count
	if len(req.Messages) > 5 {
		complexity += 0.2
	} else if len(req.Messages) > 2 {
		complexity += 0.1
	}

	// Content length
	totalLen := 0
	for _, m := range req.Messages {
		totalLen += len(m.Content)
	}

	if totalLen > 5000 {
		complexity += 0.3
	} else if totalLen > 1000 {
		complexity += 0.15
	}

	// Task type
	complexTasks := map[string]float64{
		"code_generation": 0.1,
		"analysis":        0.15,
		"code_review":     0.1,
		"debugging":       0.15,
	}

	if bonus, ok := complexTasks[req.TaskType]; ok {
		complexity += bonus
	}

	return clamp01(complexity)
}

// splitKey splits a model key into provider and model
func splitKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{"unknown", key}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
