package builtin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"devorch/internal/tool"
)

// QuestionParams defines parameters for the question tool
type QuestionParams struct {
	Question string   `json:"question"`          // question to ask the user
	Options  []string `json:"options,omitempty"` // optional list of choices
	Default  string   `json:"default,omitempty"` // default answer
}

// QuestionTool asks the user a question interactively
type QuestionTool struct {
	// Reader for input (default: os.Stdin)
	Reader *bufio.Reader
	// Writer for output (default: os.Stdout)
	Writer *os.File
}

// NewQuestionTool creates a new question tool
func NewQuestionTool() *QuestionTool {
	return &QuestionTool{
		Reader: bufio.NewReader(os.Stdin),
		Writer: os.Stdout,
	}
}

// Info returns tool metadata
func (t *QuestionTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "question",
		Description: `Ask the user a question and wait for their response.
Use this tool when you need clarification or confirmation from the user.
Supports free-form input or multiple choice options.

Examples:
  - {"question": "Which testing framework do you prefer?", "options": ["jest", "mocha", "vitest"]}
  - {"question": "Should I proceed with the refactoring?", "default": "yes"}
  - {"question": "What is the project name?"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"question": {
					"type": "string",
					"description": "The question to ask the user"
				},
				"options": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Optional list of choices for the user"
				},
				"default": {
					"type": "string",
					"description": "Default answer if user presses enter"
				}
			},
			"required": ["question"]
		}`),
	}
}

// Execute asks the user a question
func (t *QuestionTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p QuestionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	if p.Question == "" {
		return &tool.Result{
			Output:   "question is required",
			ExitCode: 1,
		}, nil
	}

	// Build prompt
	var prompt strings.Builder
	prompt.WriteString("\n")
	prompt.WriteString("📝 ")
	prompt.WriteString(p.Question)
	prompt.WriteString("\n")

	if len(p.Options) > 0 {
		prompt.WriteString("Options:\n")
		for i, opt := range p.Options {
			prompt.WriteString(fmt.Sprintf("  %d. %s\n", i+1, opt))
		}
	}

	if p.Default != "" {
		prompt.WriteString(fmt.Sprintf("[default: %s] ", p.Default))
	}
	prompt.WriteString("> ")

	// Write prompt
	fmt.Fprint(t.Writer, prompt.String())

	// Read answer with context cancellation support
	answerCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		line, err := t.Reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		answerCh <- strings.TrimSpace(line)
	}()

	select {
	case <-ctx.Ctx.Done():
		return &tool.Result{
			Output:   "Question cancelled",
			ExitCode: 1,
		}, nil
	case err := <-errCh:
		return &tool.Result{
			Output:   fmt.Sprintf("Error reading input: %s", err),
			ExitCode: 1,
		}, nil
	case answer := <-answerCh:
		// Handle empty answer with default
		if answer == "" && p.Default != "" {
			answer = p.Default
		}

		// Handle numeric selection from options
		if len(p.Options) > 0 && answer != "" {
			var idx int
			if _, err := fmt.Sscanf(answer, "%d", &idx); err == nil {
				if idx >= 1 && idx <= len(p.Options) {
					answer = p.Options[idx-1]
				}
			}
		}

		return &tool.Result{
			Output: answer,
			Metadata: map[string]any{
				"question": p.Question,
				"answer":   answer,
				"options":  p.Options,
			},
		}, nil
	}
}

// AskQuestion is a helper to ask a question programmatically
func AskQuestion(ctx context.Context, question string, options []string, defaultVal string) (string, error) {
	t := NewQuestionTool()
	params, _ := json.Marshal(QuestionParams{
		Question: question,
		Options:  options,
		Default:  defaultVal,
	})

	toolCtx := &tool.Context{Ctx: ctx}
	result, err := t.Execute(toolCtx, params)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("%s", result.Output)
	}
	return result.Output, nil
}

var _ tool.Tool = (*QuestionTool)(nil)
