package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// AgentExecutor handles autonomous agent execution
type AgentExecutor struct {
	model       *Model
	steps       []AgentStep
	currentStep int
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewAgentExecutor creates a new agent executor
func NewAgentExecutor(m *Model) *AgentExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentExecutor{
		model:       m,
		steps:       []AgentStep{},
		currentStep: 0,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// ParseStepsFromResponse extracts steps from LLM response
func (ae *AgentExecutor) ParseStepsFromResponse(response string) []AgentStep {
	steps := []AgentStep{}

	// Parse numbered steps (1., 2., 3., etc.)
	stepRegex := regexp.MustCompile(`(?m)^\s*(\d+)\.\s*\*?\*?(.+?)\*?\*?\s*$`)
	matches := stepRegex.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			stepNum := len(steps) + 1
			description := strings.TrimSpace(match[2])

			// Clean up markdown formatting
			description = strings.ReplaceAll(description, "**", "")
			description = strings.ReplaceAll(description, "*", "")

			step := AgentStep{
				Number:      stepNum,
				Description: description,
				Status:      StepPending,
			}
			steps = append(steps, step)
		}
	}

	// If no numbered steps found, try to parse bullet points
	if len(steps) == 0 {
		bulletRegex := regexp.MustCompile(`(?m)^\s*[-*]\s+(.+)$`)
		matches = bulletRegex.FindAllStringSubmatch(response, -1)

		for _, match := range matches {
			if len(match) >= 2 {
				stepNum := len(steps) + 1
				description := strings.TrimSpace(match[1])

				step := AgentStep{
					Number:      stepNum,
					Description: description,
					Status:      StepPending,
				}
				steps = append(steps, step)
			}
		}
	}

	return steps
}

// ExecuteStep executes a single step
func (ae *AgentExecutor) ExecuteStep(step *AgentStep) error {
	step.Status = StepRunning

	// Simple execution: just simulate the step
	// In a real implementation, this would:
	// 1. Parse the step description
	// 2. Determine what tool to use
	// 3. Execute the tool
	// 4. Capture the result

	// For now, we'll use the LLM to "execute" the step
	active := GetActiveProvider()

	// Build execution prompt
	prompt := fmt.Sprintf(`You are executing step %d of a multi-step task.

Step: %s

Please:
1. Execute this step
2. Provide clear output of what was done
3. If any files were created/modified, mention them
4. If any errors occurred, explain them

Keep your response concise and factual.`, step.Number, step.Description)

	messages := []Message{
		{Role: "system", Content: "You are an autonomous agent executing tasks."},
		{Role: "user", Content: prompt},
	}

	ctx, cancel := context.WithTimeout(ae.ctx, 60*time.Second)
	defer cancel()

	result, err := ChatWithProvider(ctx, active.Provider, active.Model, messages)
	if err != nil {
		step.Status = StepFailed
		step.Error = err
		return err
	}

	step.Status = StepComplete
	step.Result = result

	return nil
}

// ExecuteAll executes all steps sequentially
func (ae *AgentExecutor) ExecuteAll() error {
	for i := range ae.steps {
		select {
		case <-ae.ctx.Done():
			return fmt.Errorf("execution cancelled")
		default:
			err := ae.ExecuteStep(&ae.steps[i])
			if err != nil {
				return fmt.Errorf("step %d failed: %w", ae.steps[i].Number, err)
			}
			ae.currentStep = i + 1
		}
	}
	return nil
}

// Cancel cancels the execution
func (ae *AgentExecutor) Cancel() {
	ae.cancel()
}

// GetProgress returns current progress
func (ae *AgentExecutor) GetProgress() (completed, total int) {
	completed = 0
	total = len(ae.steps)
	for _, step := range ae.steps {
		if step.Status == StepComplete {
			completed++
		}
	}
	return completed, total
}

// agentExecuteSteps initiates agent execution
func (m *Model) agentExecuteSteps(response string) tea.Cmd {
	// Parse steps from response
	executor := NewAgentExecutor(m)
	steps := executor.ParseStepsFromResponse(response)

	if len(steps) == 0 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "⚠️  Could not parse steps from response. Please provide a numbered list of steps.",
		})
		return nil
	}

	m.agentSteps = steps
	m.agentCurrentStep = 0

	// Show confirmation
	m.ShowConfirmCustom(
		"Execute Agent Steps?",
		fmt.Sprintf("Found %d steps. Execute them automatically?", len(steps)),
		"Execute",
		"Cancel",
		func(confirmed bool) tea.Cmd {
			if confirmed {
				return m.startAgentExecution(executor)
			}
			return nil
		},
	)

	return nil
}

// startAgentExecution starts executing agent steps
func (m *Model) startAgentExecution(executor *AgentExecutor) tea.Cmd {
	executor.steps = m.agentSteps

	return func() tea.Msg {
		// Execute in background
		go func() {
			for i := range executor.steps {
				// Update model state
				m.agentCurrentStep = i
				m.agentSteps[i].Status = StepRunning

				// Execute step
				err := executor.ExecuteStep(&executor.steps[i])

				// Update model state
				m.agentSteps[i] = executor.steps[i]

				if err != nil {
					// Send error message
					break
				}
			}
		}()

		return AgentExecutionStartMsg{
			TotalSteps: len(executor.steps),
		}
	}
}

// AgentExecutionStartMsg indicates agent execution has started
type AgentExecutionStartMsg struct {
	TotalSteps int
}

// AgentStepCompleteMsg indicates a step has completed
type AgentStepCompleteMsg struct {
	StepNumber int
	Result     string
	Error      error
}

// renderAgentProgress renders agent execution progress
func (m Model) renderAgentProgress() string {
	var sb strings.Builder

	sb.WriteString(m.theme.Title.Render("🤖 Agent Execution") + "\n\n")

	if len(m.agentSteps) == 0 {
		sb.WriteString("No steps to execute.\n")
		return sb.String()
	}

	completed := 0
	for _, step := range m.agentSteps {
		if step.Status == StepComplete {
			completed++
		}
	}

	// Progress bar
	progress := float64(completed) / float64(len(m.agentSteps)) * 100
	sb.WriteString(fmt.Sprintf("Progress: %d/%d (%.0f%%)\n\n", completed, len(m.agentSteps), progress))

	// Steps
	for _, step := range m.agentSteps {
		statusIcon := step.Status.String()

		statusStyle := m.theme.Subtle
		switch step.Status {
		case StepRunning:
			statusStyle = m.theme.Accent
		case StepComplete:
			statusStyle = m.theme.Success
		case StepFailed:
			statusStyle = m.theme.Error
		}

		sb.WriteString(statusStyle.Render(fmt.Sprintf("%s Step %d: %s\n", statusIcon, step.Number, step.Description)))

		if step.Result != "" {
			// Show result with indentation
			resultLines := strings.Split(step.Result, "\n")
			for _, line := range resultLines {
				if line != "" {
					sb.WriteString(m.theme.Subtle.Render(fmt.Sprintf("  %s\n", line)))
				}
			}
		}

		if step.Error != nil {
			sb.WriteString(m.theme.Error.Render(fmt.Sprintf("  Error: %v\n", step.Error)))
		}

		sb.WriteString("\n")
	}

	if completed == len(m.agentSteps) {
		sb.WriteString(m.theme.Success.Render("✅ All steps completed!\n"))
	}

	return sb.String()
}
