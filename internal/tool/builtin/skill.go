package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"devorch/internal/tool"
)

// Skill represents a reusable skill definition
type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Prompt      string            `json:"prompt"`     // System prompt or instructions
	Tools       []string          `json:"tools"`      // Tools this skill can use
	Parameters  []SkillParam      `json:"parameters"` // Input parameters
	Examples    []SkillExample    `json:"examples"`   // Usage examples
	Metadata    map[string]string `json:"metadata"`
}

// SkillParam defines a skill parameter
type SkillParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

// SkillExample shows how to use a skill
type SkillExample struct {
	Input    map[string]any `json:"input"`
	Expected string         `json:"expected"`
}

// SkillParams defines parameters for the skill tool
type SkillParams struct {
	Name   string         `json:"name"`             // skill name to invoke
	Params map[string]any `json:"params,omitempty"` // parameters to pass
	List   bool           `json:"list,omitempty"`   // list available skills
}

// SkillTool invokes custom skills
type SkillTool struct {
	mu        sync.RWMutex
	skills    map[string]*Skill
	skillsDir string
}

// NewSkillTool creates a new skill tool
func NewSkillTool() *SkillTool {
	homeDir, _ := os.UserHomeDir()
	skillsDir := filepath.Join(homeDir, ".devorch", "skills")

	t := &SkillTool{
		skills:    make(map[string]*Skill),
		skillsDir: skillsDir,
	}

	// Load skills on creation
	_ = t.loadSkills()

	return t
}

// Info returns tool metadata
func (t *SkillTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "skill",
		Description: `Invoke a custom skill. Skills are reusable prompt templates
that can be invoked with parameters.

Skills are loaded from ~/.devorch/skills/ directory.
Each skill is a JSON file with: name, description, prompt, tools, parameters.

Examples:
  - {"list": true}                              # List available skills
  - {"name": "code_review", "params": {"file": "main.go"}}
  - {"name": "refactor", "params": {"pattern": "extract-function"}}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Name of the skill to invoke"
				},
				"params": {
					"type": "object",
					"description": "Parameters to pass to the skill"
				},
				"list": {
					"type": "boolean",
					"description": "List available skills"
				}
			}
		}`),
	}
}

// Execute invokes a skill
func (t *SkillTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p SkillParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	// List skills
	if p.List {
		return t.listSkills()
	}

	if p.Name == "" {
		return &tool.Result{
			Output:   "name is required (or use list: true to see available skills)",
			ExitCode: 1,
		}, nil
	}

	// Get skill
	t.mu.RLock()
	skill, ok := t.skills[p.Name]
	t.mu.RUnlock()

	if !ok {
		// Try to reload skills
		_ = t.loadSkills()

		t.mu.RLock()
		skill, ok = t.skills[p.Name]
		t.mu.RUnlock()

		if !ok {
			return &tool.Result{
				Output:   fmt.Sprintf("Skill not found: %s\nUse {\"list\": true} to see available skills.", p.Name),
				ExitCode: 1,
			}, nil
		}
	}

	// Validate required parameters
	for _, param := range skill.Parameters {
		if param.Required {
			if _, exists := p.Params[param.Name]; !exists {
				return &tool.Result{
					Output:   fmt.Sprintf("Missing required parameter: %s", param.Name),
					ExitCode: 1,
				}, nil
			}
		}
	}

	// Render the skill prompt with parameters
	renderedPrompt := t.renderPrompt(skill.Prompt, p.Params)

	// Build output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Skill: %s\n\n", skill.Name))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", skill.Description))
	sb.WriteString("## Rendered Prompt\n\n")
	sb.WriteString(renderedPrompt)
	sb.WriteString("\n\n")

	if len(skill.Tools) > 0 {
		sb.WriteString("## Available Tools\n\n")
		for _, toolName := range skill.Tools {
			sb.WriteString(fmt.Sprintf("- %s\n", toolName))
		}
	}

	return &tool.Result{
		Output: sb.String(),
		Metadata: map[string]any{
			"skill_name":      skill.Name,
			"rendered_prompt": renderedPrompt,
			"tools":           skill.Tools,
			"params":          p.Params,
		},
	}, nil
}

func (t *SkillTool) loadSkills() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Create skills directory if not exists
	if err := os.MkdirAll(t.skillsDir, 0755); err != nil {
		return err
	}

	// Create example skill if none exist
	entries, err := os.ReadDir(t.skillsDir)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		t.createExampleSkill()
	}

	// Load all JSON files
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(t.skillsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var skill Skill
		if err := json.Unmarshal(data, &skill); err != nil {
			continue
		}

		if skill.Name == "" {
			skill.Name = strings.TrimSuffix(entry.Name(), ".json")
		}

		t.skills[skill.Name] = &skill
	}

	return nil
}

func (t *SkillTool) createExampleSkill() {
	example := &Skill{
		Name:        "code_review",
		Description: "Review code for issues, best practices, and improvements",
		Prompt: `Please review the following code and provide feedback on:
1. Code quality and readability
2. Potential bugs or issues
3. Performance considerations
4. Best practices
5. Suggested improvements

File: {{file}}
{{#if language}}Language: {{language}}{{/if}}

Focus areas: {{#if focus}}{{focus}}{{else}}general review{{/if}}`,
		Tools: []string{"read", "grep", "glob"},
		Parameters: []SkillParam{
			{Name: "file", Type: "string", Description: "File to review", Required: true},
			{Name: "language", Type: "string", Description: "Programming language", Required: false},
			{Name: "focus", Type: "string", Description: "Areas to focus on", Required: false},
		},
		Examples: []SkillExample{
			{
				Input:    map[string]any{"file": "main.go", "focus": "error handling"},
				Expected: "Code review focusing on error handling patterns",
			},
		},
		Metadata: map[string]string{
			"author":  "DevOrch",
			"version": "1.0.0",
		},
	}

	data, _ := json.MarshalIndent(example, "", "  ")
	_ = os.WriteFile(filepath.Join(t.skillsDir, "code_review.json"), data, 0644)
}

func (t *SkillTool) renderPrompt(template string, params map[string]any) string {
	result := template

	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	// Simple conditional handling
	// {{#if key}}content{{/if}} -> show content if key exists and is truthy
	for key, value := range params {
		ifStart := fmt.Sprintf("{{#if %s}}", key)
		ifEnd := "{{/if}}"

		for {
			startIdx := strings.Index(result, ifStart)
			if startIdx == -1 {
				break
			}

			afterStart := startIdx + len(ifStart)
			endIdx := strings.Index(result[afterStart:], ifEnd)
			if endIdx == -1 {
				break
			}

			content := result[afterStart : afterStart+endIdx]

			// Check if value is truthy
			show := false
			switch v := value.(type) {
			case bool:
				show = v
			case string:
				show = v != ""
			case nil:
				show = false
			default:
				show = true
			}

			var replacement string
			if show {
				replacement = content
			}

			result = result[:startIdx] + replacement + result[afterStart+endIdx+len(ifEnd):]
		}
	}

	// Remove any remaining unmatched conditionals
	for {
		ifStart := strings.Index(result, "{{#if ")
		if ifStart == -1 {
			break
		}
		ifEnd := strings.Index(result[ifStart:], "{{/if}}")
		if ifEnd == -1 {
			break
		}
		result = result[:ifStart] + result[ifStart+ifEnd+7:]
	}

	return result
}

func (t *SkillTool) listSkills() (*tool.Result, error) {
	// Reload skills
	_ = t.loadSkills()

	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.skills) == 0 {
		return &tool.Result{
			Output: fmt.Sprintf("No skills found.\n\nCreate skills in: %s\n\nExample skill file:\n%s",
				t.skillsDir,
				`{
  "name": "my_skill",
  "description": "Description of skill",
  "prompt": "Template with {{param}}",
  "tools": ["read", "write"],
  "parameters": [
    {"name": "param", "type": "string", "required": true}
  ]
}`),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString("# Available Skills\n\n")
	sb.WriteString(fmt.Sprintf("Location: %s\n\n", t.skillsDir))

	for name, skill := range t.skills {
		sb.WriteString(fmt.Sprintf("## %s\n", name))
		sb.WriteString(fmt.Sprintf("%s\n\n", skill.Description))

		if len(skill.Parameters) > 0 {
			sb.WriteString("**Parameters:**\n")
			for _, p := range skill.Parameters {
				required := ""
				if p.Required {
					required = " (required)"
				}
				sb.WriteString(fmt.Sprintf("- `%s` (%s)%s: %s\n", p.Name, p.Type, required, p.Description))
			}
			sb.WriteString("\n")
		}

		if len(skill.Tools) > 0 {
			sb.WriteString(fmt.Sprintf("**Tools:** %s\n\n", strings.Join(skill.Tools, ", ")))
		}
	}

	return &tool.Result{
		Output: sb.String(),
		Metadata: map[string]any{
			"skill_count": len(t.skills),
			"skills_dir":  t.skillsDir,
		},
	}, nil
}

// RegisterSkill programmatically registers a skill
func (t *SkillTool) RegisterSkill(skill *Skill) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skills[skill.Name] = skill
}

var _ tool.Tool = (*SkillTool)(nil)
