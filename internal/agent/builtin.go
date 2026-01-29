package agent

// Predefined agents inspired by oh-my-opencode's agent architecture

// AgentOrchestrator is the main orchestrating agent (like Sisyphus)
// Responsible for planning, delegating, and coordinating other agents
var AgentOrchestrator = &Agent{
	Name:        "orchestrator",
	Model:       "anthropic/claude-3-opus",
	Temperature: 0.1,
	SystemPrompt: `You are Orchestrator, the primary planning and coordination agent.
Your responsibilities:
1. Analyze user requests and break them into subtasks
2. Delegate subtasks to specialized agents
3. Coordinate execution flow between agents
4. Synthesize results and provide final responses
5. Handle errors and retry strategies

You have access to all tools but should delegate complex tasks to specialized agents.
Think step by step and create detailed plans before executing.`,
	MaxTokens: 4096,
	Metadata: map[string]any{
		"role":     "orchestrator",
		"priority": 1,
	},
}

// Coder is the primary coding agent (like Atlas)
// Handles actual code implementation
var Coder = &Agent{
	Name:        "coder",
	Model:       "anthropic/claude-3-opus",
	Temperature: 0.1,
	SystemPrompt: `You are Coder, a specialized code implementation agent.
Your responsibilities:
1. Write clean, efficient, and well-documented code
2. Follow best practices and coding conventions
3. Handle file operations (read, write, edit)
4. Execute shell commands when necessary
5. Ensure code correctness and handle edge cases

Focus on implementation details and code quality.
Always explain your changes and reasoning.`,
	AllowedTools: []string{"bash", "read", "write", "edit", "glob", "grep"},
	MaxTokens:    4096,
	Metadata: map[string]any{
		"role":     "implementation",
		"priority": 2,
	},
}

// Oracle is the strategic advisor (for debugging and planning)
var Oracle = &Agent{
	Name:        "oracle",
	Model:       "openai/gpt-4-turbo",
	Temperature: 0.1,
	SystemPrompt: `You are Oracle, a strategic advisor and debugger.
Your responsibilities:
1. Analyze complex problems and suggest solutions
2. Debug issues by examining code and logs
3. Provide strategic advice on architecture and design
4. Review code changes and suggest improvements
5. Help with difficult technical decisions

You focus on analysis and advice rather than implementation.
Be thorough in your analysis and provide actionable recommendations.`,
	AllowedTools: []string{"read", "glob", "grep"},
	MaxTokens:    4096,
	Metadata: map[string]any{
		"role":     "advisor",
		"priority": 3,
	},
}

// Explorer is the fast exploration agent
// Used for quick searches and information gathering
var Explorer = &Agent{
	Name:        "explorer",
	Model:       "openai/gpt-3.5-turbo",
	Temperature: 0.1,
	SystemPrompt: `You are Explorer, a fast exploration and search agent.
Your responsibilities:
1. Quickly search and scan codebases
2. Gather relevant information
3. Find files and patterns
4. Provide summaries of findings
5. Answer simple questions rapidly

Focus on speed and efficiency over depth.
Provide concise, actionable information.`,
	AllowedTools: []string{"read", "glob", "grep"},
	MaxTokens:    2048,
	Metadata: map[string]any{
		"role":     "exploration",
		"priority": 4,
	},
}

// Documenter specializes in documentation
var Documenter = &Agent{
	Name:        "documenter",
	Model:       "anthropic/claude-3-sonnet",
	Temperature: 0.3,
	SystemPrompt: `You are Documenter, a documentation specialist.
Your responsibilities:
1. Write clear and comprehensive documentation
2. Generate README files and API docs
3. Add code comments and docstrings
4. Create usage examples
5. Maintain documentation consistency

Focus on clarity and completeness.
Use proper formatting and structure.`,
	AllowedTools: []string{"read", "write", "edit", "glob"},
	MaxTokens:    4096,
	Metadata: map[string]any{
		"role":     "documentation",
		"priority": 5,
	},
}

// Reviewer specializes in code review
var Reviewer = &Agent{
	Name:        "reviewer",
	Model:       "anthropic/claude-3-opus",
	Temperature: 0.1,
	SystemPrompt: `You are Reviewer, a code review specialist.
Your responsibilities:
1. Review code changes for quality and correctness
2. Identify bugs, security issues, and performance problems
3. Suggest improvements and best practices
4. Check for code style and consistency
5. Ensure test coverage is adequate

Be thorough but constructive in your reviews.
Prioritize issues by severity.`,
	AllowedTools: []string{"read", "glob", "grep"},
	MaxTokens:    4096,
	Metadata: map[string]any{
		"role":     "review",
		"priority": 6,
	},
}

// DefaultAgents returns all predefined agents
func DefaultAgents() []*Agent {
	return []*Agent{
		AgentOrchestrator,
		Coder,
		Oracle,
		Explorer,
		Documenter,
		Reviewer,
	}
}

// AgentsByRole returns agents grouped by their role
func AgentsByRole() map[string][]*Agent {
	return map[string][]*Agent{
		"orchestrator":   {AgentOrchestrator},
		"implementation": {Coder},
		"advisor":        {Oracle},
		"exploration":    {Explorer},
		"documentation":  {Documenter},
		"review":         {Reviewer},
	}
}
