// Package tui provides terminal user interface components using bubbletea.
package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"devorch/internal/auth"
	"devorch/internal/session"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// globalSessionStore is the shared session store
var globalSessionStore *session.Store

// initSessionStore initializes the global session store
func initSessionStore() error {
	if globalSessionStore != nil {
		return nil
	}

	// Default session directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	sessionDir := filepath.Join(homeDir, ".config", "devorch", "sessions")

	store, err := session.NewStore(sessionDir)
	if err != nil {
		return err
	}
	globalSessionStore = store
	return nil
}

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewModeChat ViewMode = iota
	ViewModeSessions
	ViewModeSettings
	ViewModeHelp
	ViewModeCommands         // Slash command palette (main command groups)
	ViewModeSubCommands      // Subcommand selection
	ViewModeModelSelect      // Model selection
	ViewModeProviderSelect   // Provider selection
	ViewModeThemeSelect      // Theme selection
	ViewModeLogin            // OAuth login
	ViewModeMCP              // MCP server management
	ViewModeSetup            // Auto setup wizard
	ViewModeAgentSelect      // Agent mode selection
	ViewModeInstallSelect    // Model installation selection
	ViewModeLanguageSelect   // Language selection
	ViewModeConnect          // OpenCode-style provider connect
	ViewModeConfirm          // Confirmation dialog
	ViewModeProgress         // Progress indicator
	ViewModeMultiModelSelect // Multi-model selection (Phase 3)
	ViewModeCompare          // Multi-model response comparison (Phase 3)
)

// WorkMode represents the work/task mode (Phase 2)
type WorkMode int

const (
	WorkModeAsk   WorkMode = iota // Quick Q&A mode
	WorkModeEdit                  // Code editing mode
	WorkModeAgent                 // Autonomous agent mode
	WorkModePlan                  // Task planning mode
)

// String returns the string representation of WorkMode
func (w WorkMode) String() string {
	switch w {
	case WorkModeAsk:
		return "Ask"
	case WorkModeEdit:
		return "Edit"
	case WorkModeAgent:
		return "Agent"
	case WorkModePlan:
		return "Plan"
	default:
		return "Ask"
	}
}

// Icon returns the emoji icon for the work mode
func (w WorkMode) Icon() string {
	switch w {
	case WorkModeAsk:
		return "💬"
	case WorkModeEdit:
		return "✏️"
	case WorkModeAgent:
		return "🤖"
	case WorkModePlan:
		return "📋"
	default:
		return "💬"
	}
}

// Model represents the main TUI application state.
type Model struct {
	// Core state
	mode     ViewMode
	ready    bool
	width    int
	height   int
	quitting bool
	err      error

	// Components
	input    textinput.Model
	viewport viewport.Model
	spinner  spinner.Model

	// Chat state
	messages    []Message
	isStreaming bool
	sessionID   string
	sessionName string

	// Sessions list
	sessions        []SessionInfo
	selectedIdx     int
	sessionsFocused bool

	// Command palette
	showCommandPreview bool
	commandPalette     CommandPaletteModel

	// Interactive command autocomplete (OpenCode style)
	showCommandAutocomplete bool
	commandFilter           string
	commandSelectedIdx      int
	filteredCommands        []SlashCommand

	// Subcommand selection
	selectedMainCommand string           // e.g., "session", "model"
	subcommandList      []SubcommandItem // List of subcommands to choose from
	subcommandIdx       int              // Selected index in subcommand list

	// Selection lists (for models, themes, providers)
	selectionList      []string
	selectionIdx       int
	selectionTitle     string
	installableModels  []InstallableModel // For /install selection
	selectedModelIdxs  map[int]bool       // Multi-selection for install
	installSystemSpecs SystemSpecs        // System specs for install

	// Connect mode (OpenCode style)
	connectProviders    []ConnectProvider // All providers for /connect
	connectSelectedIdx  int               // Selected provider index
	connectFilter       string            // Search filter
	connectFilteredList []ConnectProvider // Filtered providers
	connectInputMode    bool              // true = API key input mode
	connectSearchMode   bool              // true = search mode active
	apiKeyInput         textinput.Model   // API key input field
	providerSearchInput textinput.Model   // Provider search input

	// Provider selection improvements
	providerFilter     string         // Current filter for provider selection
	filteredProviders  []ProviderInfo // Filtered provider list
	showProviderSearch bool           // Show search input in provider selection

	// Display options (OpenCode style)
	showDetails  bool // Show token count, timing, model info
	thinkingMode bool // Extended thinking for supported models

	// Navigation stack for back button (Phase 1)
	viewStack []ViewMode
	viewData  map[ViewMode]interface{} // Store state for each view

	// Confirm dialog (Phase 1)
	confirmTitle    string
	confirmMessage  string
	confirmYes      string // Default: "Yes"
	confirmNo       string // Default: "No"
	confirmSelected int    // 0 = Yes, 1 = No
	confirmCallback func(bool) tea.Cmd

	// Progress indicator (Phase 1)
	progressTitle   string
	progressPercent float64
	progressBytes   int64
	progressTotal   int64
	progressSpeed   string
	progressETA     string
	progressStatus  string // Additional status text
	progressErr     error

	// Work mode system (Phase 2)
	workMode         WorkMode          // Current work mode (Ask/Edit/Agent/Plan)
	workModeContext  map[string]string // Mode-specific context data
	editContextFiles []string          // Files in context for Edit mode
	agentSteps       []AgentStep       // Steps for Agent mode
	agentCurrentStep int               // Current step in Agent mode
	planAnalysis     *TaskAnalysis     // Analysis for Plan mode

	// Multi-model system (Phase 3)
	multiModelEnabled bool                      // Enable multi-model mode
	selectedModels    []ModelSelection          // Selected models for multi-model
	modelResponses    map[string]*ModelResponse // Responses from each model
	showCompareView   bool                      // Show side-by-side comparison
	compareScrollIdx  int                       // Scroll index for compare view
	modelRatings      map[string]ResponseRating // User ratings for responses

	// Preset system (Phase 4)
	presetManager *PresetManager // Preset manager

	// Theme
	theme Theme
}

// Message represents a chat message.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
	Tokens  int
}

// SessionInfo represents session metadata for listing.
type SessionInfo struct {
	ID        string
	Name      string
	UpdatedAt string
	Messages  int
}

// SubcommandItem represents a subcommand option
type SubcommandItem struct {
	Name        string
	Description string
	Handler     func(m *Model) tea.Cmd
}

// AgentStep represents a step in agent execution (Phase 2)
type AgentStep struct {
	Number      int
	Description string
	Tool        string
	Args        map[string]interface{}
	Status      StepStatus
	Result      string
	Error       error
}

// StepStatus represents the status of an agent step
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepComplete
	StepFailed
)

// String returns the string representation of StepStatus
func (s StepStatus) String() string {
	switch s {
	case StepPending:
		return "⋯"
	case StepRunning:
		return "⏳"
	case StepComplete:
		return "✓"
	case StepFailed:
		return "✗"
	default:
		return "⋯"
	}
}

// TaskAnalysis represents task analysis for Plan mode (Phase 2)
type TaskAnalysis struct {
	Goal          string
	Steps         []PlannedStep
	EstimatedTime int // in minutes
	Risks         []Risk
	Dependencies  []string
	FilesAffected []string
}

// PlannedStep represents a planned step in task analysis
type PlannedStep struct {
	Number      int
	Description string
	Estimated   int // in minutes
	Risk        RiskLevel
	Files       []string
}

// Risk represents a potential risk in task execution
type Risk struct {
	Level       RiskLevel
	Description string
	Mitigation  string
}

// RiskLevel represents the risk level
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
)

// String returns the string representation of RiskLevel
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "Low"
	case RiskMedium:
		return "Medium"
	case RiskHigh:
		return "High"
	default:
		return "Low"
	}
}

// Color returns the color for the risk level
func (r RiskLevel) Color() string {
	switch r {
	case RiskLow:
		return "green"
	case RiskMedium:
		return "yellow"
	case RiskHigh:
		return "red"
	default:
		return "white"
	}
}

// ================== Phase 3: Multi-Model Types ==================

// ModelSelection represents a selected model for multi-model mode
type ModelSelection struct {
	Provider    string
	Model       string
	DisplayName string
	Selected    bool
}

// ModelResponse represents a response from a model in multi-model mode
type ModelResponse struct {
	Provider    string
	Model       string
	DisplayName string
	Content     string
	Tokens      int
	Duration    int64 // in milliseconds
	Error       error
	InProgress  bool
	StartTime   time.Time
}

// ResponseRating represents user rating for a model response
type ResponseRating struct {
	ModelKey string // "provider:model"
	Rating   int    // 1 = thumbs down, 2 = thumbs up, 0 = no rating
	Comment  string
}

// ================== End Phase 3 Types ==================

// New creates a new TUI model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Type your message or / for commands..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	theme := DefaultTheme()

	// Initialize preset manager
	presetManager, err := NewPresetManager()
	if err != nil {
		// Non-fatal, continue without preset functionality
		presetManager = nil
	}

	// Initialize search inputs
	providerSearchInput := textinput.New()
	providerSearchInput.Placeholder = "Search providers..."
	providerSearchInput.CharLimit = 50
	providerSearchInput.Width = 40

	apiKeyInput := textinput.New()
	apiKeyInput.Placeholder = "Enter your API key..."
	apiKeyInput.EchoMode = textinput.EchoPassword
	apiKeyInput.CharLimit = 200
	apiKeyInput.Width = 50

	return Model{
		mode:                ViewModeChat,
		input:               ti,
		spinner:             sp,
		theme:               theme,
		commandPalette:      NewCommandPalette(theme),
		viewStack:           []ViewMode{},
		viewData:            make(map[ViewMode]interface{}),
		confirmYes:          "Yes",
		confirmNo:           "No",
		confirmSelected:     0,
		workMode:            WorkModeAsk, // Default to Ask mode
		workModeContext:     make(map[string]string),
		editContextFiles:    []string{},
		agentSteps:          []AgentStep{},
		multiModelEnabled:   false,
		selectedModels:      []ModelSelection{},
		modelResponses:      make(map[string]*ModelResponse),
		showCompareView:     false,
		compareScrollIdx:    0,
		modelRatings:        make(map[string]ResponseRating),
		presetManager:       presetManager,
		providerSearchInput: providerSearchInput,
		apiKeyInput:         apiKeyInput,
		filteredProviders:   []ProviderInfo{},
		messages: []Message{
			{Role: "system", Content: "Welcome to DevOrch! Type / for commands or start chatting.\n\n💬 Mode: Ask - Quick questions and answers"},
		},
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 3
		inputHeight := 3
		verticalMargins := headerHeight + footerHeight + inputHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargins)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
		}

		m.input.Width = msg.Width - 4
		m.viewport.SetContent(m.renderMessages())

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case StreamChunkMsg:
		m.handleStreamChunk(msg)
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		// If this chunk has a NextCmd, execute it to get the next chunk
		if msg.NextCmd != nil && !msg.Done {
			cmds = append(cmds, func() tea.Msg { return msg.NextCmd() })
		}
		// If marked as done, stop streaming
		if msg.Done {
			m.isStreaming = false
		}

	case StreamDoneMsg:
		m.isStreaming = false

		// If in Agent mode, check if we should parse and execute steps
		if m.workMode == WorkModeAgent && len(m.messages) > 0 {
			lastMsg := m.messages[len(m.messages)-1]
			if lastMsg.Role == "assistant" {
				// Parse steps from response and offer to execute
				cmd = m.agentExecuteSteps(lastMsg.Content)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}

		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case ErrorMsg:
		m.err = msg.Err
		m.isStreaming = false

	case SessionsLoadedMsg:
		m.sessions = msg.Sessions

	case SessionCreatedMsg:
		m.sessionID = msg.SessionID
		m.sessionName = msg.SessionName
		m.messages = []Message{
			{Role: "system", Content: fmt.Sprintf("📝 New session created: %s", msg.SessionName)},
		}

	case SessionMessagesLoadedMsg:
		m.messages = msg.Messages

	case *ModelInstallStartMsg:
		// Start installation in background
		return m, m.startModelInstallation(msg.Provider, msg.ModelID)

	case *ModelInstallProgressMsg:
		// Update progress indicator
		m.UpdateProgress(msg.Percent, msg.Bytes, msg.Total, msg.Speed, msg.ETA, msg.Status)
		return m, nil

	case *ModelInstallCompleteMsg:
		if msg.Success {
			// Installation successful
			m.CloseProgress()
			active := GetActiveProvider()
			SetActiveProvider(active.Provider, msg.ModelID)
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: fmt.Sprintf("✅ Model '%s' installed successfully!\n\nYou can now use it for your tasks.", msg.ModelID),
			})
			m.mode = ViewModeChat
		} else {
			// Installation failed
			m.SetProgressError(msg.Error)
		}
		return m, nil

	case ModelResponseMsg:
		// Handle multi-model response
		m.handleModelResponse(msg)
		return m, nil

	case AgentExecutionStartMsg:
		// Agent execution started
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("🤖 Starting agent execution: %d steps", msg.TotalSteps),
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case AgentStepCompleteMsg:
		// Update step status
		if msg.StepNumber > 0 && msg.StepNumber <= len(m.agentSteps) {
			step := &m.agentSteps[msg.StepNumber-1]
			if msg.Error != nil {
				step.Status = StepFailed
				step.Error = msg.Error
			} else {
				step.Status = StepComplete
				step.Result = msg.Result
			}

			// Update display
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}
		return m, nil
	}

	// Update input
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle special modes first
	switch m.mode {
	case ViewModeCommands:
		return m.handleCommandPaletteKey(msg)
	case ViewModeSubCommands:
		return m.handleSubCommandKey(msg)
	case ViewModeThemeSelect, ViewModeModelSelect, ViewModeProviderSelect, ViewModeLogin, ViewModeAgentSelect, ViewModeLanguageSelect:
		return m.handleSelectionKey(msg)
	case ViewModeInstallSelect:
		return m.handleInstallSelectionKey(msg)
	case ViewModeConnect:
		return m.handleConnectKey(msg)
	case ViewModeConfirm:
		return m.handleConfirmKey(msg)
	case ViewModeProgress:
		return m.handleProgressKey(msg)
	case ViewModeMultiModelSelect:
		return m.handleMultiModelSelectKey(msg)
	case ViewModeCompare:
		return m.handleCompareKey(msg)
	}

	// Handle interactive command autocomplete
	if m.showCommandAutocomplete {
		return m.handleCommandAutocompleteKey(msg)
	}

	// Global Esc handler for navigation stack
	if msg.String() == "esc" && len(m.viewStack) > 0 {
		m.PopView()
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		m.quitting = true
		return m, tea.Quit

	case "ctrl+n":
		// New session
		return m, m.newSession()

	case "ctrl+s":
		// Toggle sessions view
		if m.mode == ViewModeSessions {
			m.mode = ViewModeChat
		} else {
			m.mode = ViewModeChat
		}
		return m, nil

	case "ctrl+h":
		// Toggle help
		if m.mode == ViewModeHelp {
			m.mode = ViewModeChat
		} else {
			m.mode = ViewModeHelp
		}
		return m, nil

	case "ctrl+p":
		// Open command palette
		m.mode = ViewModeCommands
		return m, nil

	case "ctrl+1":
		// Switch to Ask mode
		return m, m.switchWorkMode(WorkModeAsk)

	case "ctrl+2":
		// Switch to Edit mode
		return m, m.switchWorkMode(WorkModeEdit)

	case "ctrl+3":
		// Switch to Agent mode
		return m, m.switchWorkMode(WorkModeAgent)

	case "ctrl+4":
		// Switch to Plan mode
		return m, m.switchWorkMode(WorkModePlan)

	case "tab":
		// Autocomplete slash command (when preview is showing)
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			// Select the highlighted command
			selected := m.filteredCommands[m.commandSelectedIdx]
			m.input.SetValue("/" + selected.Name)
			m.input.CursorEnd()
			m.showCommandAutocomplete = false
			m.commandSelectedIdx = 0
		}
		return m, nil

	case "enter":
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			// Execute the selected command directly
			selected := m.filteredCommands[m.commandSelectedIdx]
			m.input.SetValue("")
			m.showCommandAutocomplete = false
			m.commandSelectedIdx = 0

			// Execute the command
			result := selected.Handler(&m)
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			return m, result
		}
		if m.mode == ViewModeChat && !m.isStreaming {
			return m.sendMessage()
		}
		if m.mode == ViewModeSessions {
			return m.selectSession()
		}

	case "up":
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			if m.commandSelectedIdx > 0 {
				m.commandSelectedIdx--
			}
			return m, nil
		}
		if m.mode == ViewModeSessions && m.selectedIdx > 0 {
			m.selectedIdx--
			return m, nil
		}

	case "down":
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			if m.commandSelectedIdx < len(m.filteredCommands)-1 {
				m.commandSelectedIdx++
			}
			return m, nil
		}
		if m.mode == ViewModeSessions && m.selectedIdx < len(m.sessions)-1 {
			m.selectedIdx++
			return m, nil
		}

	case "esc":
		if m.showCommandAutocomplete {
			m.showCommandAutocomplete = false
			m.commandSelectedIdx = 0
			return m, nil
		}
		if m.mode != ViewModeChat {
			m.mode = ViewModeChat
			m.showCommandPreview = false
			return m, nil
		}
	}

	// IMPORTANT: Forward all other key events to the input component
	// This allows text input to work properly
	if m.mode == ViewModeChat {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Check if input starts with / to show command autocomplete
	input := m.input.Value()
	if strings.HasPrefix(input, "/") && len(input) >= 1 {
		m.showCommandAutocomplete = true
		m.commandFilter = input
		m.filteredCommands = FilterCommands(input)
		// Reset selection if filter changed
		if m.commandSelectedIdx >= len(m.filteredCommands) {
			m.commandSelectedIdx = 0
		}
	} else {
		m.showCommandAutocomplete = false
		m.commandSelectedIdx = 0
	}
	m.showCommandPreview = m.showCommandAutocomplete // Keep compatibility

	return m, tea.Batch(cmds...)
}

// handleCommandAutocompleteKey handles keys when autocomplete is showing
func (m Model) handleCommandAutocompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.commandSelectedIdx > 0 {
			m.commandSelectedIdx--
		}
		return m, nil
	case "down":
		if m.commandSelectedIdx < len(m.filteredCommands)-1 {
			m.commandSelectedIdx++
		}
		return m, nil
	case "tab", "enter":
		if len(m.filteredCommands) > 0 {
			selected := m.filteredCommands[m.commandSelectedIdx]
			if msg.String() == "enter" {
				// Execute immediately
				m.input.SetValue("")
				m.showCommandAutocomplete = false
				m.commandSelectedIdx = 0
				result := selected.Handler(&m)
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				return m, result
			} else {
				// Tab: fill in command name
				m.input.SetValue("/" + selected.Name)
				m.input.CursorEnd()
				m.showCommandAutocomplete = false
				m.commandSelectedIdx = 0
			}
		}
		return m, nil
	case "esc":
		m.showCommandAutocomplete = false
		m.commandSelectedIdx = 0
		return m, nil
	default:
		// Forward to input for typing
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)

		// Update filtered commands
		input := m.input.Value()
		if strings.HasPrefix(input, "/") {
			m.filteredCommands = FilterCommands(input)
			if m.commandSelectedIdx >= len(m.filteredCommands) {
				m.commandSelectedIdx = 0
			}
		} else {
			m.showCommandAutocomplete = false
		}
		return m, cmd
	}
}

// handleCommandPaletteKey handles keys in command palette mode.
func (m Model) handleCommandPaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = ViewModeChat
		return m, nil
	case "enter":
		if cmd := m.commandPalette.SelectedCommand(); cmd != nil {
			m.mode = ViewModeChat
			return m, cmd.Handler(&m)
		}
	case "up", "k":
		if m.commandPalette.list.Index() > 0 {
			m.commandPalette.list.CursorUp()
		}
	case "down", "j":
		m.commandPalette.list.CursorDown()
	}

	var cmd tea.Cmd
	m.commandPalette, cmd = m.commandPalette.Update(msg)
	return m, cmd
}

// handleSelectionKey handles keys in selection list mode.
func (m Model) handleSelectionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle provider search mode first
	if m.mode == ViewModeProviderSelect && m.showProviderSearch {
		switch msg.String() {
		case "esc":
			m.showProviderSearch = false
			m.providerSearchInput.SetValue("")
			m.providerFilter = ""
			m.filterProviders()
			return m, nil
		case "enter":
			// Apply filter and exit search mode
			m.showProviderSearch = false
			m.providerFilter = m.providerSearchInput.Value()
			m.filterProviders()
			return m, nil
		default:
			// Update search input and filter
			var cmd tea.Cmd
			m.providerSearchInput, cmd = m.providerSearchInput.Update(msg)
			m.providerFilter = m.providerSearchInput.Value()
			m.filterProviders()
			return m, cmd
		}
	}
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = ViewModeChat
		return m, nil
	case "enter":
		if m.selectionIdx >= 0 && m.selectionIdx < len(m.selectionList) {
			selected := m.selectionList[m.selectionIdx]
			switch m.mode {
			case ViewModeThemeSelect:
				m.theme = GetTheme(selected)
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("Theme changed to: %s", selected),
				})
			case ViewModeModelSelect:
				// Extract model ID (remove status indicator if present)
				modelID := strings.TrimSuffix(selected, " ✓")
				modelID = strings.TrimSpace(modelID)
				active := GetActiveProvider()

				// Phase 1: Check if model is installed
				if !isModelInstalled(active.Provider, modelID) {
					// Show confirmation dialog for installation
					m.ShowConfirm(
						"Install Model?",
						fmt.Sprintf("Model '%s' is not installed.\n\nWould you like to download and install it now?", modelID),
						func(confirmed bool) tea.Cmd {
							if confirmed {
								return m.installModelWithProgress(active.Provider, modelID)
							}
							// User declined, go back to model selection
							m.mode = ViewModeModelSelect
							return nil
						},
					)
					return m, nil
				}

				// Model is already installed, just set it
				SetActiveProvider(active.Provider, modelID)
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("Model changed to: %s\nProvider: %s", modelID, active.Provider),
				})
				m.mode = ViewModeChat
			case ViewModeProviderSelect:
				// Use filtered providers instead of all providers
				if m.selectionIdx < len(m.filteredProviders) {
					p := m.filteredProviders[m.selectionIdx]
					// Get first model for this provider
					models := GetModelsForProvider(p.Name)
					defaultModel := ""
					for _, model := range models {
						if model.IsPrimary {
							defaultModel = model.ID
							break
						}
					}
					if defaultModel == "" && len(models) > 0 {
						defaultModel = models[0].ID
					}
					SetActiveProvider(p.Name, defaultModel)
					m.messages = append(m.messages, Message{
						Role:    "system",
						Content: fmt.Sprintf("Provider changed to: %s\nDefault model: %s", p.DisplayName, defaultModel),
					})
				}
			case ViewModeAgentSelect:
				// Extract agent mode
				agentModes := []string{"coder", "chat", "researcher", "writer", "task"}
				if m.selectionIdx < len(agentModes) {
					agentMode := agentModes[m.selectionIdx]
					m.messages = append(m.messages, Message{
						Role:    "system",
						Content: fmt.Sprintf("🤖 Agent mode: %s\n\nYour AI assistant will now focus on %s tasks.", agentMode, agentMode),
					})
				}
			case ViewModeLanguageSelect:
				m.handleLanguageSelection(selected)
			case ViewModeLogin:
				// Open browser for the selected provider
				providerMap := map[int]string{
					0: "anthropic",
					1: "openai",
					2: "google",
					3: "openrouter",
					4: "groq",
					5: "github",
				}
				if provider, ok := providerMap[m.selectionIdx]; ok {
					if err := OpenBrowserForLogin(provider); err != nil {
						m.messages = append(m.messages, Message{
							Role:    "system",
							Content: fmt.Sprintf("Failed to open browser: %v", err),
						})
					} else {
						envVar := ""
						switch provider {
						case "anthropic":
							envVar = "ANTHROPIC_API_KEY"
						case "openai":
							envVar = "OPENAI_API_KEY"
						case "google":
							envVar = "GOOGLE_API_KEY"
						case "openrouter":
							envVar = "OPENROUTER_API_KEY"
						case "groq":
							envVar = "GROQ_API_KEY"
						case "github":
							envVar = "GITHUB_TOKEN"
						}
						m.messages = append(m.messages, Message{
							Role:    "system",
							Content: fmt.Sprintf("🌐 Opening browser for %s...\n\nAfter getting your API key, set it with:\n  export %s=your_key_here\n\nOr add to ~/.config/devorch/config.yaml", provider, envVar),
						})
					}
				}
			}
			m.mode = ViewModeChat
		}
		return m, nil
	case "/":
		// Enter search mode for provider selection
		if m.mode == ViewModeProviderSelect {
			m.showProviderSearch = true
			m.providerSearchInput.Focus()
			return m, nil
		}
	case "up", "k":
		if m.selectionIdx > 0 {
			m.selectionIdx--
		}
	case "down", "j":
		if m.selectionIdx < len(m.selectionList)-1 {
			m.selectionIdx++
		}
	}
	return m, nil
}

// handleLanguageSelection applies the selected language
func (m Model) handleLanguageSelection(selection string) {
	langMap := map[string]string{
		"English (en)":  "en",
		"한국어 (ko)":      "ko",
		"日本語 (ja)":      "ja",
		"中文 (zh)":       "zh",
		"Español (es)":  "es",
		"Français (fr)": "fr",
		"Deutsch (de)":  "de",
	}

	if code, ok := langMap[selection]; ok {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("🌐 Language changed to: %s\n\nNote: Full i18n support coming soon!", code),
		})
	}
}

// sendMessage sends the current input as a message.
func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}

	// Handle commands
	if strings.HasPrefix(text, "/") {
		return m.handleCommand(text)
	}

	// Add user message (original text)
	m.messages = append(m.messages, Message{
		Role:    "user",
		Content: text,
	})

	m.input.SetValue("")
	m.isStreaming = true
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	// Build mode-specific prompt
	enhancedPrompt := m.buildPromptForMode(text)

	// Check if multi-model mode is enabled
	if m.multiModelEnabled {
		// Switch to compare view
		m.mode = ViewModeCompare
		return m, m.sendToMultipleModels(enhancedPrompt)
	}

	// Return command to send message to LLM (single model)
	return m, m.sendToLLM(enhancedPrompt)
}

// handleCommand processes slash commands.
func (m Model) handleCommand(cmdStr string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return m, nil
	}

	m.input.SetValue("")
	m.showCommandPreview = false

	// Find and execute the command
	cmdName := strings.TrimPrefix(parts[0], "/")
	if cmd := FindCommand(cmdName); cmd != nil {
		result := cmd.Handler(&m)

		// Handle special mode transitions - these are now handled in command handlers
		// Just update viewport
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, result
	}

	// Unknown command
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", parts[0]),
	})

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.ready {
		return "Initializing..."
	}

	switch m.mode {
	case ViewModeSessions:
		return m.viewSessions()
	case ViewModeHelp:
		return m.viewHelp()
	case ViewModeCommands:
		return m.viewCommandPalette()
	case ViewModeSubCommands:
		return m.viewSubCommands()
	case ViewModeThemeSelect, ViewModeModelSelect, ViewModeProviderSelect, ViewModeLogin, ViewModeAgentSelect, ViewModeLanguageSelect:
		return m.viewSelectionList()
	case ViewModeInstallSelect:
		return m.viewInstallSelection()
	case ViewModeConnect:
		return m.viewConnect()
	case ViewModeSettings:
		return m.viewSettings()
	case ViewModeSetup:
		return m.viewSetup()
	case ViewModeMCP:
		return m.viewMCP()
	case ViewModeConfirm:
		return m.viewConfirm()
	case ViewModeProgress:
		return m.viewProgress()
	case ViewModeMultiModelSelect:
		return m.viewMultiModelSelect()
	case ViewModeCompare:
		return m.viewCompare()
	default:
		return m.viewChat()
	}
}

// viewChat renders the chat view.
func (m Model) viewChat() string {
	header := m.renderHeader()
	content := m.viewport.View()

	// Show interactive command autocomplete (OpenCode style)
	if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
		autocomplete := m.renderCommandAutocomplete()
		autocompleteBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")). // Bright blue
			Padding(0, 1).
			Width(m.width - 4).
			Render(autocomplete)
		content = content + "\n" + autocompleteBox
	}

	input := m.renderInput()
	footer := m.renderFooter()

	return fmt.Sprintf("%s\n%s\n%s\n%s", header, content, input, footer)
}

// renderCommandAutocomplete renders the interactive command list
func (m Model) renderCommandAutocomplete() string {
	var sb strings.Builder

	maxShow := 10
	totalCommands := len(m.filteredCommands)

	// Calculate scroll window based on selected index
	startIdx := 0
	if m.commandSelectedIdx >= maxShow {
		startIdx = m.commandSelectedIdx - maxShow + 1
	}
	endIdx := startIdx + maxShow
	if endIdx > totalCommands {
		endIdx = totalCommands
	}

	// Header with scroll indicator
	if startIdx > 0 {
		sb.WriteString(m.theme.Accent.Render("Commands") + m.theme.Subtle.Render(" (↑↓ to select, Enter to run, Tab to complete)") + "\n")
		sb.WriteString(m.theme.Subtle.Render("  ↑ more above") + "\n")
	} else {
		sb.WriteString(m.theme.Accent.Render("Commands") + m.theme.Subtle.Render(" (↑↓ to select, Enter to run, Tab to complete)") + "\n")
	}

	for i := startIdx; i < endIdx; i++ {
		cmd := m.filteredCommands[i]
		cursor := "  "
		nameStyle := m.theme.Normal
		descStyle := m.theme.Subtle

		if i == m.commandSelectedIdx {
			cursor = "▶ "
			nameStyle = m.theme.Selected
			descStyle = m.theme.Selected
		}

		// Format: ▶ /command - description
		cmdLine := fmt.Sprintf("%s/%s", cursor, cmd.Name)
		descLine := fmt.Sprintf(" - %s", cmd.Description)

		// Truncate if too long
		maxLineLen := m.width - 10
		if maxLineLen > 0 && len(cmdLine)+len(descLine) > maxLineLen {
			if maxLineLen-len(cmdLine) > 6 {
				descLine = descLine[:maxLineLen-len(cmdLine)-3] + "..."
			}
		}

		sb.WriteString(nameStyle.Render(cmdLine) + descStyle.Render(descLine) + "\n")
	}

	// Show scroll indicator at bottom
	if endIdx < totalCommands {
		sb.WriteString(m.theme.Subtle.Render(fmt.Sprintf("  ↓ %d more below", totalCommands-endIdx)))
	}

	return sb.String()
}

// viewCommandPalette renders the command palette.
func (m Model) viewCommandPalette() string {
	var b strings.Builder

	title := m.theme.Title.Render("⌘ Command Palette")
	b.WriteString(title + "\n\n")

	b.WriteString(m.commandPalette.View())

	b.WriteString("\n\n" + m.theme.Subtle.Render("Enter: Select | Esc: Close | ↑/↓: Navigate | Type to filter"))

	return b.String()
}

// viewSelectionList renders a selection list (themes, models, providers).
func (m Model) viewSelectionList() string {
	var b strings.Builder

	title := m.theme.Title.Render("📋 " + m.selectionTitle)
	b.WriteString(title + "\n\n")

	// Show search input for provider selection
	if m.mode == ViewModeProviderSelect {
		if m.showProviderSearch {
			// Search mode active
			b.WriteString(m.theme.Accent.Render("🔍 Search: "))
			b.WriteString(m.providerSearchInput.View())
			b.WriteString("\n")
			b.WriteString(m.theme.Subtle.Render("Press Enter to apply filter, Esc to cancel"))
			b.WriteString("\n\n")
		} else if m.providerFilter != "" {
			// Show current filter
			b.WriteString(m.theme.Subtle.Render(fmt.Sprintf("🔍 Filtered: '%s' - Press / to search again", m.providerFilter)))
			b.WriteString("\n\n")
		} else {
			// Show search hint
			b.WriteString(m.theme.Subtle.Render("🔍 Press / to search providers"))
			b.WriteString("\n\n")
		}
	}

	if len(m.selectionList) == 0 {
		if m.mode == ViewModeProviderSelect && m.providerFilter != "" {
			b.WriteString(m.theme.Warning.Render("No providers found matching: " + m.providerFilter))
			b.WriteString("\n\n")
			b.WriteString(m.theme.Subtle.Render("Press / to search again or Esc to clear filter"))
		} else {
			b.WriteString(m.theme.Subtle.Render("No items available."))
		}
	} else {
		maxShow := 15
		startIdx := 0
		if m.selectionIdx >= maxShow {
			startIdx = m.selectionIdx - maxShow + 1
		}
		endIdx := startIdx + maxShow
		if endIdx > len(m.selectionList) {
			endIdx = len(m.selectionList)
		}

		for i := startIdx; i < endIdx; i++ {
			cursor := "  "
			style := m.theme.Normal
			if i == m.selectionIdx {
				cursor = "▶ "
				style = m.theme.Selected
			}

			// Enhanced display for providers
			if m.mode == ViewModeProviderSelect && i < len(m.filteredProviders) {
				p := m.filteredProviders[i]
				statusIcon := "○" // Not connected
				statusColor := m.theme.Warning
				if p.AuthStatus == "authenticated" {
					statusIcon = "✓" // Connected
					statusColor = m.theme.Success
				}

				kindTag := ""
				if p.Kind == "local" {
					kindTag = m.theme.Accent.Render(" [LOCAL]")
				} else {
					kindTag = m.theme.Subtle.Render(" [CLOUD]")
				}

				line := cursor + statusColor.Render(statusIcon) + " " + p.DisplayName + kindTag
				if i == m.selectionIdx {
					line = style.Render(line)
				}
				b.WriteString(line + "\n")
			} else {
				// Default display for other selection types
				b.WriteString(style.Render(cursor+m.selectionList[i]) + "\n")
			}
		}

		if len(m.selectionList) > maxShow {
			b.WriteString(m.theme.Subtle.Render(fmt.Sprintf("\n  ... %d more items", len(m.selectionList)-maxShow)))
		}
	}

	b.WriteString("\n\n" + m.theme.Subtle.Render("Enter: Select | Esc: Cancel | ↑/↓: Navigate"))

	return b.String()
}

// viewInstallSelection renders the model installation selection view
func (m Model) viewInstallSelection() string {
	var b strings.Builder

	// Header with system specs
	title := m.theme.Title.Render("📦 Model Installation")
	b.WriteString(title + "\n\n")

	specs := m.installSystemSpecs
	b.WriteString(m.theme.Accent.Render("🖥️  System Specs:") + "\n")
	b.WriteString(fmt.Sprintf("  RAM: %.1f GB  |  CPU: %d cores  |  %s\n\n", specs.RAM, specs.CPUCores, specs.Tier))

	// Count selections
	selectedCount := 0
	var totalSize float64
	for idx := range m.selectedModelIdxs {
		if idx < len(m.installableModels) && !m.installableModels[idx].Installed {
			selectedCount++
			// Parse size (e.g., "1.3GB" -> 1.3)
			sizeStr := m.installableModels[idx].Size
			var size float64
			fmt.Sscanf(sizeStr, "%fGB", &size)
			totalSize += size
		}
	}

	b.WriteString(m.theme.Accent.Render(fmt.Sprintf("Selected: %d models (%.1f GB total)\n\n", selectedCount, totalSize)))

	// Group models by category
	currentCategory := ""
	maxShow := 15
	startIdx := 0
	if m.selectionIdx >= maxShow {
		startIdx = m.selectionIdx - maxShow + 1
	}
	endIdx := startIdx + maxShow
	if endIdx > len(m.installableModels) {
		endIdx = len(m.installableModels)
	}

	for i := startIdx; i < endIdx; i++ {
		model := m.installableModels[i]

		// Show category header
		if model.Category != currentCategory {
			if currentCategory != "" {
				b.WriteString("\n")
			}
			b.WriteString(m.theme.Accent.Render(model.Category) + "\n")
			currentCategory = model.Category
		}

		// Selection indicator
		checkbox := "[ ]"
		if model.Installed {
			checkbox = m.theme.Success.Render("[✓]")
		} else if m.selectedModelIdxs[i] {
			checkbox = m.theme.Accent.Render("[◉]")
		}

		// Cursor
		cursor := "  "
		style := m.theme.Normal
		if i == m.selectionIdx {
			cursor = "▶ "
			style = m.theme.Selected
		}

		// Format line
		line := fmt.Sprintf("%s%s %s (%s) - %s",
			cursor,
			checkbox,
			model.Name,
			model.Size,
			model.Description,
		)

		// Truncate if too long
		maxLen := m.width - 4
		if len(line) > maxLen && maxLen > 20 {
			line = line[:maxLen-3] + "..."
		}

		b.WriteString(style.Render(line) + "\n")
	}

	// Scroll indicator
	if endIdx < len(m.installableModels) {
		b.WriteString(m.theme.Subtle.Render(fmt.Sprintf("\n  ↓ %d more models below\n", len(m.installableModels)-endIdx)))
	}

	// Footer
	b.WriteString("\n" + m.theme.Subtle.Render("Space: Toggle | A: All recommended | N: None | Enter: Install | Esc: Cancel"))

	return b.String()
}

// viewSettings renders the settings view.
func (m Model) viewSettings() string {
	return m.theme.Title.Render("⚙ Settings") + `

` + m.theme.Accent.Render("General") + `
  • Theme: ` + m.theme.Normal.Render("default") + `
  • Language: ` + m.theme.Normal.Render("en") + `

` + m.theme.Accent.Render("AI") + `
  • Default Provider: ` + m.theme.Normal.Render("Anthropic") + `
  • Default Model: ` + m.theme.Normal.Render("claude-sonnet-4-20250514") + `

` + m.theme.Accent.Render("Local") + `
  • Ollama: ` + m.theme.Success.Render("Installed") + `
  • Auto-download models: ` + m.theme.Normal.Render("Yes") + `

` + m.theme.Subtle.Render("Press Esc to close")
}

// viewLogin renders the login view.
func (m Model) viewLogin() string {
	return m.theme.Title.Render("🔐 Login") + `

` + m.theme.Accent.Render("Select a provider to authenticate:") + `

  1. Anthropic (Claude)
  2. OpenAI (GPT)
  3. Google (Gemini)
  4. GitHub Copilot
  5. Azure OpenAI

` + m.theme.Subtle.Render("Note: Local models (Ollama) don't require authentication.") + `

` + m.theme.Subtle.Render("Press number to login or Esc to cancel")
}

// viewSetup renders the setup wizard.
func (m Model) viewSetup() string {
	return m.theme.Title.Render("🚀 Auto Setup Wizard") + `

` + m.theme.Accent.Render("Detecting system...") + `

  ` + m.theme.Success.Render("✓") + ` OS: macOS (arm64)
  ` + m.theme.Success.Render("✓") + ` Memory: 16 GB
  ` + m.theme.Success.Render("✓") + ` GPU: Apple Silicon (Metal)
  ` + m.theme.Success.Render("✓") + ` Tier: High

` + m.theme.Accent.Render("Recommended Models:") + `
  • llama3.2:13b (primary)
  • qwen2.5-coder:14b (coding)
  • deepseek-coder:6.7b (fast)

` + m.theme.Warning.Render("This will download ~20GB of models.") + `

` + m.theme.Subtle.Render("Press Enter to start or Esc to cancel")
}

// viewMCP renders the MCP management view.
func (m Model) viewMCP() string {
	return m.theme.Title.Render("🔌 MCP Server Management") + `

` + m.theme.Accent.Render("Active Servers:") + `
  ` + m.theme.Subtle.Render("No MCP servers connected.") + `

` + m.theme.Accent.Render("Available Servers:") + `
  • filesystem - File system access
  • git - Git operations
  • github - GitHub API
  • memory - Persistent memory

` + m.theme.Subtle.Render("Press number to toggle or Esc to close")
}

// viewSessions renders the sessions list view.
func (m Model) viewSessions() string {
	var b strings.Builder

	title := m.theme.Title.Render("📋 Sessions")
	b.WriteString(title + "\n\n")

	if len(m.sessions) == 0 {
		b.WriteString(m.theme.Subtle.Render("No sessions found."))
	} else {
		for i, s := range m.sessions {
			cursor := "  "
			style := m.theme.Normal
			if i == m.selectedIdx {
				cursor = "▶ "
				style = m.theme.Selected
			}

			line := fmt.Sprintf("%s%s (%d messages) - %s",
				cursor, s.Name, s.Messages, s.UpdatedAt)
			b.WriteString(style.Render(line) + "\n")
		}
	}

	b.WriteString("\n" + m.theme.Subtle.Render("Enter: Select | Esc: Back | ↑/↓: Navigate"))

	return b.String()
}

// viewHelp renders the help view.
func (m Model) viewHelp() string {
	help := `
╔══════════════════════════════════════════════════════════════════╗
║                       DevOrch Help                               ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║  SLASH COMMANDS (type / then use ↑↓ to select)                  ║
║  ───────────────────────────────────────────────                 ║
║  Session:   /new /clear /reset /save /export /share /compact    ║
║  Project:   /init /files /grep /ls /diff /git                   ║
║  Model:     /model /models /provider /providers /agent          ║
║  Context:   /context /memory /add                               ║
║  Settings:  /settings /theme /themes /config                    ║
║  Auth:      /login /logout /auth                                ║
║  Tools:     /tools /mcp /lsp                                    ║
║  System:    /status /setup /doctor /version /install /bench     ║
║  Edit:      /undo /redo                                         ║
║  Help:      /help /quit                                         ║
║                                                                  ║
║  KEYBOARD SHORTCUTS                                              ║
║  ──────────────────                                              ║
║  Ctrl+N     New session                                         ║
║  Ctrl+S     Toggle sessions panel                               ║
║  Ctrl+H     Toggle this help                                    ║
║  Ctrl+P     Open command palette                                ║
║  Ctrl+C/Q   Quit application                                    ║
║  ↑/↓        Navigate in command list / selection lists          ║
║  Enter      Send message / Select / Execute command             ║
║  Tab        Complete command name                               ║
║  Esc        Close dialogs / Cancel                              ║
║                                                                  ║
║  TIPS                                                            ║
║  ────                                                            ║
║  • Type / to see all commands with ↑↓ navigation                ║
║  • Use /init to analyze your project                            ║
║  • Use /model to switch AI models                               ║
║  • Use /compact when context gets full                          ║
║                                                                  ║
║  Press Esc to close this help                                   ║
╚══════════════════════════════════════════════════════════════════╝
`
	return m.theme.Help.Render(help)
}

// renderHeader renders the header bar.
func (m Model) renderHeader() string {
	title := "DevOrch"
	if m.sessionName != "" {
		title += " - " + m.sessionName
	}

	// Add work mode indicator
	modeIndicator := fmt.Sprintf("%s %s", m.workMode.Icon(), m.workMode.String())

	status := ""
	if m.isStreaming {
		status = m.spinner.View() + " Thinking..."
	} else {
		status = modeIndicator
	}

	left := m.theme.Title.Render(title)
	right := m.theme.Subtle.Render(status)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return left + strings.Repeat(" ", gap) + right
}

// renderMessages renders all chat messages.
func (m Model) renderMessages() string {
	var b strings.Builder

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			b.WriteString(m.theme.UserMsg.Render("You: " + msg.Content))
		case "assistant":
			b.WriteString(m.theme.AssistantMsg.Render("AI: " + msg.Content))
		case "system":
			b.WriteString(m.theme.SystemMsg.Render("⚙ " + msg.Content))
		}
		b.WriteString("\n\n")
	}

	// Show agent execution progress if in Agent mode and steps are active
	if m.workMode == WorkModeAgent && len(m.agentSteps) > 0 {
		b.WriteString(m.renderAgentProgress())
		b.WriteString("\n\n")
	}

	return b.String()
}

// renderInput renders the input area.
func (m Model) renderInput() string {
	return m.theme.InputBox.Render(m.input.View())
}

// renderFooter renders the footer bar.
func (m Model) renderFooter() string {
	shortcuts := "Ctrl+N: New | Ctrl+S: Sessions | Ctrl+H: Help | Ctrl+1/2/3/4: Mode | Ctrl+C: Quit"

	// Debug info (temporary)
	debug := fmt.Sprintf(" | mode=%d ac=%v sel=%d", m.mode, m.showCommandAutocomplete, len(m.selectionList))
	return m.theme.Footer.Render(shortcuts + debug)
}

// Message types for tea.Cmd

// StreamChunkMsg represents a streaming chunk from the LLM.
type StreamChunkMsg struct {
	Content string
	Done    bool
	NextCmd func() tea.Msg
}

// StreamDoneMsg indicates streaming is complete.
type StreamDoneMsg struct{}

// ErrorMsg represents an error.
type ErrorMsg struct {
	Err error
}

// SessionsLoadedMsg indicates sessions have been loaded.
type SessionsLoadedMsg struct {
	Sessions []SessionInfo
}

// Commands

// ActiveProvider holds the currently selected provider and model
type ActiveProvider struct {
	Provider string
	Model    string
}

// globalActiveProvider tracks the active provider/model
var globalActiveProvider = ActiveProvider{
	Provider: "ollama",
	Model:    "tinyllama:latest", // default to lightweight installed model
}

// SetActiveProvider sets the active provider
func SetActiveProvider(provider, model string) {
	globalActiveProvider.Provider = provider
	globalActiveProvider.Model = model
}

// GetActiveProvider returns the active provider
func GetActiveProvider() ActiveProvider {
	return globalActiveProvider
}

// filterProviders filters the provider list based on current filter
func (m *Model) filterProviders() {
	allProviders := GetAvailableProviders()
	if m.providerFilter == "" {
		m.filteredProviders = allProviders
		// Update selection list for display
		m.selectionList = make([]string, 0, len(allProviders))
		for _, p := range allProviders {
			status := "○"
			if p.AuthStatus == "authenticated" {
				status = "✓"
			}
			kind := ""
			if p.Kind == "local" {
				kind = " (local)"
			}
			m.selectionList = append(m.selectionList, fmt.Sprintf("%s%s %s", p.DisplayName, kind, status))
		}
	} else {
		filter := strings.ToLower(m.providerFilter)
		m.filteredProviders = []ProviderInfo{}
		m.selectionList = []string{}
		for _, p := range allProviders {
			if strings.Contains(strings.ToLower(p.DisplayName), filter) ||
				strings.Contains(strings.ToLower(p.Name), filter) {
				m.filteredProviders = append(m.filteredProviders, p)
				status := "○"
				if p.AuthStatus == "authenticated" {
					status = "✓"
				}
				kind := ""
				if p.Kind == "local" {
					kind = " (local)"
				}
				m.selectionList = append(m.selectionList, fmt.Sprintf("%s%s %s", p.DisplayName, kind, status))
			}
		}
	}
	// Reset selection index if it's out of bounds
	if m.selectionIdx >= len(m.selectionList) {
		m.selectionIdx = 0
	}
}

func (m Model) sendToLLM(text string) tea.Cmd {
	active := GetActiveProvider()

	// Convert messages to the right format (skip system welcome message)
	var chatMsgs []Message
	for _, msg := range m.messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Welcome to DevOrch") {
			continue
		}
		chatMsgs = append(chatMsgs, msg)
	}
	chatMsgs = append(chatMsgs, Message{Role: "user", Content: text})

	// Use streaming for Ollama, non-streaming for others
	if active.Provider == "ollama" {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		// Don't defer cancel here - let it be cancelled when streaming completes
		_ = cancel
		return m.sendToLLMStreaming(ctx, active, chatMsgs)
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Use the unified ChatWithProvider function for non-streaming
		response, err := ChatWithProvider(ctx, active.Provider, active.Model, chatMsgs)
		if err != nil {
			// Provider-specific error messages
			switch active.Provider {
			case "ollama":
				if strings.Contains(err.Error(), "model") || strings.Contains(err.Error(), "not found") {
					return StreamChunkMsg{
						Content: fmt.Sprintf("❌ Model '%s' not found.\n\nPlease install it first:\n  ollama pull %s\n\nOr run 'devorch setup' to install essential models.", active.Model, active.Model),
					}
				}
				return StreamChunkMsg{
					Content: fmt.Sprintf("❌ Ollama error: %v\n\nMake sure Ollama is running: ollama serve", err),
				}
			case "anthropic", "openai", "google", "openrouter", "groq":
				if strings.Contains(err.Error(), "not set") {
					var envVar string
					switch active.Provider {
					case "anthropic":
						envVar = "ANTHROPIC_API_KEY"
					case "openai":
						envVar = "OPENAI_API_KEY"
					case "google":
						envVar = "GOOGLE_API_KEY"
					case "openrouter":
						envVar = "OPENROUTER_API_KEY"
					case "groq":
						envVar = "GROQ_API_KEY"
					}
					return StreamChunkMsg{
						Content: fmt.Sprintf("❌ %s not set.\n\nUse /login to get your API key, then set:\n  export %s=your_key_here\n\nOr add it to ~/.config/devorch/config.yaml", envVar, envVar),
					}
				}
				return StreamChunkMsg{
					Content: fmt.Sprintf("❌ %s error: %v", active.Provider, err),
				}
			default:
				return StreamChunkMsg{
					Content: fmt.Sprintf("❌ Error: %v", err),
				}
			}
		}

		return StreamChunkMsg{Content: response}
	}
}

// sendToLLMStreaming handles streaming responses from Ollama
func (m Model) sendToLLMStreaming(ctx context.Context, active ActiveProvider, chatMsgs []Message) tea.Cmd {
	return func() tea.Msg {
		client := NewOllamaClient()

		// Start assistant message immediately
		return tea.Batch(
			func() tea.Msg {
				// Channel to send chunks
				chunkChan := make(chan string, 100)
				doneChan := make(chan error, 1)

				// Start streaming in goroutine
				go func() {
					defer close(chunkChan)
					defer close(doneChan)

					err := client.ChatStream(ctx, active.Model, chatMsgs, func(chunk string) {
						select {
						case chunkChan <- chunk:
						case <-ctx.Done():
							return
						}
					})

					if err != nil {
						doneChan <- err
					} else {
						doneChan <- nil
					}
				}()

				// Return a command that continuously reads from the channel
				return streamNextChunk(chunkChan, doneChan, active.Model)
			},
		)()
	}
}

// streamNextChunk reads the next chunk from the channel
func streamNextChunk(chunkChan chan string, doneChan chan error, model string) tea.Msg {
	select {
	case chunk, ok := <-chunkChan:
		if !ok {
			// Channel closed, check for errors
			select {
			case err := <-doneChan:
				if err != nil {
					if strings.Contains(err.Error(), "model") || strings.Contains(err.Error(), "not found") {
						return StreamChunkMsg{
							Content: fmt.Sprintf("❌ Model '%s' not found.\n\nPlease install it first:\n  ollama pull %s\n\nOr run 'devorch setup' to install essential models.", model, model),
							Done:    true,
						}
					}
					return StreamChunkMsg{
						Content: fmt.Sprintf("❌ Ollama error: %v\n\nMake sure Ollama is running: ollama serve", err),
						Done:    true,
					}
				}
				return StreamDoneMsg{}
			default:
				return StreamDoneMsg{}
			}
		}
		// Return this chunk and schedule next read
		return StreamChunkMsg{Content: chunk, NextCmd: func() tea.Msg { return streamNextChunk(chunkChan, doneChan, model) }}
	case err := <-doneChan:
		if err != nil {
			if strings.Contains(err.Error(), "model") || strings.Contains(err.Error(), "not found") {
				return StreamChunkMsg{
					Content: fmt.Sprintf("❌ Model '%s' not found.\n\nPlease install it first:\n  ollama pull %s\n\nOr run 'devorch setup' to install essential models.", model, model),
					Done:    true,
				}
			}
			return StreamChunkMsg{
				Content: fmt.Sprintf("❌ Ollama error: %v\n\nMake sure Ollama is running: ollama serve", err),
				Done:    true,
			}
		}
		return StreamDoneMsg{}
	}
}

func (m *Model) handleStreamChunk(msg StreamChunkMsg) {
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
		// Append to existing assistant message
		m.messages[len(m.messages)-1].Content += msg.Content
	} else {
		// Create new assistant message
		m.messages = append(m.messages, Message{
			Role:    "assistant",
			Content: msg.Content,
		})
	}
}

func (m Model) newSession() tea.Cmd {
	return func() tea.Msg {
		// Initialize session store if needed
		if err := initSessionStore(); err != nil {
			return StreamChunkMsg{Content: fmt.Sprintf("❌ Failed to init session store: %v", err)}
		}

		// Create new session
		ctx := context.Background()
		cwd, _ := os.Getwd()
		sessID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
		sess, err := globalSessionStore.Create(ctx, sessID, "New Chat", cwd)
		if err != nil {
			return StreamChunkMsg{Content: fmt.Sprintf("❌ Failed to create session: %v", err)}
		}

		return SessionCreatedMsg{
			SessionID:   sess.ID,
			SessionName: sess.Name,
		}
	}
}

// SessionCreatedMsg is sent when a new session is created
type SessionCreatedMsg struct {
	SessionID   string
	SessionName string
}

func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		// Initialize session store if needed
		if err := initSessionStore(); err != nil {
			return SessionsLoadedMsg{
				Sessions: []SessionInfo{},
			}
		}

		// Load sessions from store
		ctx := context.Background()
		summaries, err := globalSessionStore.ListSummaries(ctx, "")
		if err != nil {
			return SessionsLoadedMsg{
				Sessions: []SessionInfo{},
			}
		}

		var sessions []SessionInfo
		for _, s := range summaries {
			sessions = append(sessions, SessionInfo{
				ID:        s.ID,
				Name:      s.Name,
				UpdatedAt: s.UpdatedAt.Format("2006-01-02 15:04"),
				Messages:  s.Messages,
			})
		}

		// If no sessions, add placeholder
		if len(sessions) == 0 {
			sessions = append(sessions, SessionInfo{
				ID:        "new",
				Name:      "(No sessions - start chatting to create one)",
				UpdatedAt: time.Now().Format("2006-01-02 15:04"),
				Messages:  0,
			})
		}

		return SessionsLoadedMsg{
			Sessions: sessions,
		}
	}
}

func (m Model) selectSession() (tea.Model, tea.Cmd) {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.sessions) {
		s := m.sessions[m.selectedIdx]

		// Handle "new session" placeholder
		if s.ID == "new" {
			m.mode = ViewModeChat
			return m, m.newSession()
		}

		m.sessionID = s.ID
		m.sessionName = s.Name
		m.mode = ViewModeChat

		// Load session messages
		return m, m.loadSessionMessages(s.ID)
	}
	return m, nil
}

func (m Model) loadSessionMessages(sessionID string) tea.Cmd {
	return func() tea.Msg {
		if globalSessionStore == nil {
			return nil
		}

		ctx := context.Background()
		sess, err := globalSessionStore.Get(ctx, sessionID)
		if err != nil {
			return StreamChunkMsg{Content: fmt.Sprintf("❌ Failed to load session: %v", err)}
		}

		// Convert session messages to TUI messages
		var messages []Message
		messages = append(messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("📂 Loaded session: %s (%d messages)", sess.Name, len(sess.Messages)),
		})

		for _, msg := range sess.Messages {
			messages = append(messages, Message{
				Role:    string(msg.Role),
				Content: msg.Content,
				Tokens:  msg.Tokens,
			})
		}

		return SessionMessagesLoadedMsg{Messages: messages}
	}
}

// SessionMessagesLoadedMsg is sent when session messages are loaded
type SessionMessagesLoadedMsg struct {
	Messages []Message
}

// handleInstallSelectionKey handles keys in install selection mode
func (m Model) handleInstallSelectionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c", "q":
		m.mode = ViewModeChat
		return m, nil

	case "enter":
		// Start installation of selected models
		var toInstall []string
		for idx := range m.selectedModelIdxs {
			if idx < len(m.installableModels) && !m.installableModels[idx].Installed {
				toInstall = append(toInstall, m.installableModels[idx].ID)
			}
		}

		if len(toInstall) > 0 {
			// Start background installation
			for _, id := range toInstall {
				go func(modelID string) {
					exec.Command("ollama", "pull", modelID).Run()
				}(id)
			}

			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: fmt.Sprintf("⬇️  Installing %d models in background:\n%s\n\n💡 Check progress: ollama ps", len(toInstall), strings.Join(toInstall, ", ")),
			})
		} else {
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: "✅ No new models selected for installation.",
			})
		}

		m.mode = ViewModeChat
		return m, nil

	case "up", "k":
		if m.selectionIdx > 0 {
			m.selectionIdx--
		} else {
			m.selectionIdx = len(m.installableModels) - 1
		}
		return m, nil

	case "down", "j":
		if m.selectionIdx < len(m.installableModels)-1 {
			m.selectionIdx++
		} else {
			m.selectionIdx = 0
		}
		return m, nil

	case " ": // Space to toggle selection
		if m.selectionIdx >= 0 && m.selectionIdx < len(m.installableModels) {
			if !m.installableModels[m.selectionIdx].Installed {
				if m.selectedModelIdxs[m.selectionIdx] {
					delete(m.selectedModelIdxs, m.selectionIdx)
				} else {
					m.selectedModelIdxs[m.selectionIdx] = true
				}
			}
		}
		return m, nil

	case "a": // Select all recommended
		for i, model := range m.installableModels {
			if model.Recommended && !model.Installed {
				m.selectedModelIdxs[i] = true
			}
		}
		return m, nil

	case "n": // Clear selection
		m.selectedModelIdxs = make(map[int]bool)
		return m, nil
	}

	return m, nil
}

// viewConnect renders the OpenCode-style provider connect view
func (m Model) viewConnect() string {
	var sb strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Padding(1, 2)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 2)

	sb.WriteString(titleStyle.Render("🔗 Connect a Provider"))
	sb.WriteString("\n")
	sb.WriteString(subtitleStyle.Render("Select a provider to connect. Use ↑↓ to navigate, Enter to select, / to search"))
	sb.WriteString("\n\n")

	// If in API key input mode
	if m.connectInputMode {
		provider := m.connectFilteredList[m.connectSelectedIdx]
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
			fmt.Sprintf("Enter API key for %s:\n", provider.Name)))
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
			fmt.Sprintf("Environment variable: %s\n\n", provider.EnvVar)))
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(m.apiKeyInput.View()))
		sb.WriteString("\n\n")
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("241")).Render(
			"Press Enter to save • Esc to cancel • Press 'o' to open browser for key"))
		return sb.String()
	}

	// Search bar
	if m.connectFilter != "" {
		searchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Padding(0, 2)
		sb.WriteString(searchStyle.Render(fmt.Sprintf("🔍 %s", m.connectFilter)))
		sb.WriteString("\n\n")
	}

	// Provider list
	providers := m.connectFilteredList
	if len(providers) == 0 {
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("241")).Render(
			"No providers match your search."))
		return sb.String()
	}

	// Group by category
	popularProviders := []ConnectProvider{}
	otherProviders := []ConnectProvider{}
	for _, p := range providers {
		if p.Category == "Popular" {
			popularProviders = append(popularProviders, p)
		} else {
			otherProviders = append(otherProviders, p)
		}
	}

	// Calculate which provider is currently selected across all groups
	currentIdx := 0
	categoryHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("141")).
		Padding(0, 2)

	itemStyle := lipgloss.NewStyle().Padding(0, 2)
	selectedStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(lipgloss.Color("39")).
		Foreground(lipgloss.Color("15"))
	connectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	maxShow := 15
	startIdx := 0
	totalItems := len(providers)
	if m.connectSelectedIdx >= maxShow {
		startIdx = m.connectSelectedIdx - maxShow + 1
	}

	// Render Popular
	if len(popularProviders) > 0 && startIdx < len(popularProviders) {
		sb.WriteString(categoryHeaderStyle.Render("Popular"))
		sb.WriteString("\n")

		for i, p := range popularProviders {
			if currentIdx < startIdx {
				currentIdx++
				continue
			}
			if currentIdx-startIdx >= maxShow {
				break
			}

			name := p.Name
			suffix := ""
			if p.IsConnected {
				suffix = connectedStyle.Render(" Connected")
			}

			line := fmt.Sprintf("  %s%s", name, suffix)
			if currentIdx == m.connectSelectedIdx {
				sb.WriteString(selectedStyle.Render(fmt.Sprintf("▸ %s%s", name, suffix)))
			} else {
				sb.WriteString(itemStyle.Render(line))
			}
			sb.WriteString("\n")
			currentIdx++
			_ = i
		}
		sb.WriteString("\n")
	} else {
		currentIdx = len(popularProviders)
	}

	// Render Other
	if len(otherProviders) > 0 && currentIdx-startIdx < maxShow {
		sb.WriteString(categoryHeaderStyle.Render("Other"))
		sb.WriteString("\n")

		for i, p := range otherProviders {
			globalIdx := len(popularProviders) + i
			if globalIdx < startIdx {
				continue
			}
			if globalIdx-startIdx >= maxShow {
				break
			}

			name := p.Name
			suffix := ""
			if p.IsConnected {
				suffix = connectedStyle.Render(" Connected")
			}

			line := fmt.Sprintf("  %s%s", name, suffix)
			if globalIdx == m.connectSelectedIdx {
				sb.WriteString(selectedStyle.Render(fmt.Sprintf("▸ %s%s", name, suffix)))
			} else {
				sb.WriteString(itemStyle.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	// Scroll indicator
	if totalItems > maxShow {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("241")).Render(
			fmt.Sprintf("  %d of %d providers", m.connectSelectedIdx+1, totalItems)))
	}

	// Footer
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("241")).Render(
		"↑↓ Navigate • Enter Select • / Search • Esc Cancel"))

	return sb.String()
}

// handleConnectKey handles keys in connect mode
func (m Model) handleConnectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.connectInputMode {
		// Handle API key input mode
		switch msg.String() {
		case "esc":
			m.connectInputMode = false
			m.apiKeyInput.SetValue("")
			return m, nil
		case "enter":
			// Save API key
			apiKey := m.apiKeyInput.Value()
			if apiKey != "" {
				provider := m.connectFilteredList[m.connectSelectedIdx]
				// Save to auth store
				store := auth.GetStore()
				store.Set(provider.ID, &auth.AuthInfo{
					APIKey: apiKey,
				})
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("✓ API key saved for %s\n\nYou can now use this provider!", provider.Name),
				})
				m.connectInputMode = false
				m.mode = ViewModeChat
				return m, nil
			}
		case "ctrl+o":
			// Open browser for API key
			provider := m.connectFilteredList[m.connectSelectedIdx]
			if err := OpenBrowserForLogin(provider.ID); err != nil {
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("Failed to open browser: %v", err),
				})
			}
			return m, nil
		default:
			// Update API key input
			var cmd tea.Cmd
			m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
			return m, cmd
		}
	}

	// Handle normal navigation mode
	switch msg.String() {
	case "esc":
		m.mode = ViewModeChat
		return m, nil
	case "enter":
		if len(m.connectFilteredList) > 0 && m.connectSelectedIdx < len(m.connectFilteredList) {
			provider := m.connectFilteredList[m.connectSelectedIdx]
			
			// Handle different auth types
			switch provider.AuthType {
			case "oauth":
				// Start OAuth flow
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("🔐 Starting OAuth login for %s...\n\nThis will open your browser to complete the authentication.", provider.Name),
				})
				m.mode = ViewModeChat
				// Trigger OAuth flow
				if err := OpenBrowserForLogin(provider.ID); err != nil {
					m.messages = append(m.messages, Message{
						Role:    "system",
						Content: fmt.Sprintf("Failed to open browser: %v", err),
					})
				}
				return m, nil
			case "api_key":
				// Enter API key input mode
				m.connectInputMode = true
				m.apiKeyInput.Focus()
				m.apiKeyInput.SetValue("")
				return m, nil
			case "none":
				// No auth needed (like Ollama)
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("✓ Connected to %s\n\nNo authentication required for this provider.", provider.Name),
				})
				m.mode = ViewModeChat
				return m, nil
			}
		}
		return m, nil
	case "up", "k":
		if m.connectSelectedIdx > 0 {
			m.connectSelectedIdx--
		}
		return m, nil
	case "down", "j":
		if m.connectSelectedIdx < len(m.connectFilteredList)-1 {
			m.connectSelectedIdx++
		}
		return m, nil
	case "/":
		// Enter search mode
		m.connectSearchMode = true
		m.providerSearchInput.Focus()
		return m, nil
	}

	return m, nil
}

// handleSubCommandKey handles key events in subcommand selection mode
func (m Model) handleSubCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.subcommandIdx > 0 {
			m.subcommandIdx--
		}
		return m, nil

	case "down", "j":
		if m.subcommandIdx < len(m.subcommandList)-1 {
			m.subcommandIdx++
		}
		return m, nil

	case "enter":
		if m.subcommandIdx < len(m.subcommandList) {
			selected := m.subcommandList[m.subcommandIdx]
			m.mode = ViewModeChat
			return m, selected.Handler(&m)
		}
		return m, nil

	case "esc", "q":
		m.mode = ViewModeChat
		return m, nil
	}

	return m, nil
}

// viewSubCommands renders the subcommand selection view
func (m Model) viewSubCommands() string {
	var b strings.Builder

	title := m.theme.Title.Render(fmt.Sprintf("📋 /%s Commands", m.selectedMainCommand))
	b.WriteString(title + "\n\n")

	if len(m.subcommandList) == 0 {
		b.WriteString(m.theme.Subtle.Render("No subcommands available."))
	} else {
		for i, sub := range m.subcommandList {
			cursor := "  "
			style := m.theme.Normal
			if i == m.subcommandIdx {
				cursor = "▶ "
				style = m.theme.Selected
			}

			line := fmt.Sprintf("%s%-20s %s", cursor, sub.Name, sub.Description)
			b.WriteString(style.Render(line) + "\n")
		}
	}

	b.WriteString("\n" + m.theme.Border.Render(strings.Repeat("─", m.width-4)) + "\n")
	b.WriteString(m.theme.Subtle.Render("↑/↓: Navigate | Enter: Select | Esc: Back"))

	return b.String()
}

// OpenBrowser opens a URL in the default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

// ============================================================================
// Phase 1: Navigation Stack Functions
// ============================================================================

// PushView pushes current view to stack and switches to new view
func (m *Model) PushView(newMode ViewMode) {
	m.viewStack = append(m.viewStack, m.mode)
	m.mode = newMode
}

// PopView returns to previous view from stack
func (m *Model) PopView() bool {
	if len(m.viewStack) == 0 {
		return false
	}
	m.mode = m.viewStack[len(m.viewStack)-1]
	m.viewStack = m.viewStack[:len(m.viewStack)-1]
	return true
}

// ClearViewStack clears the navigation stack
func (m *Model) ClearViewStack() {
	m.viewStack = []ViewMode{}
}

// ============================================================================
// Phase 1: Confirm Dialog
// ============================================================================

// ShowConfirm displays a confirmation dialog
func (m *Model) ShowConfirm(title, message string, callback func(bool) tea.Cmd) {
	m.PushView(ViewModeConfirm)
	m.confirmTitle = title
	m.confirmMessage = message
	m.confirmCallback = callback
	m.confirmSelected = 0
	m.confirmYes = "Yes"
	m.confirmNo = "No"
}

// ShowConfirmCustom displays a confirmation dialog with custom button labels
func (m *Model) ShowConfirmCustom(title, message, yesLabel, noLabel string, callback func(bool) tea.Cmd) {
	m.PushView(ViewModeConfirm)
	m.confirmTitle = title
	m.confirmMessage = message
	m.confirmCallback = callback
	m.confirmSelected = 0
	m.confirmYes = yesLabel
	m.confirmNo = noLabel
}

// viewConfirm renders the confirmation dialog
func (m Model) viewConfirm() string {
	var b strings.Builder

	// Title
	title := m.theme.Title.Render(m.confirmTitle)
	b.WriteString(title + "\n\n")

	// Message
	message := m.theme.Normal.Render(m.confirmMessage)
	b.WriteString(message + "\n\n")

	// Buttons
	yesStyle := m.theme.Normal
	noStyle := m.theme.Normal

	if m.confirmSelected == 0 {
		yesStyle = m.theme.Accent.Bold(true)
	} else {
		noStyle = m.theme.Accent.Bold(true)
	}

	yesButton := yesStyle.Render(fmt.Sprintf("[ %s ]", m.confirmYes))
	noButton := noStyle.Render(fmt.Sprintf("[ %s ]", m.confirmNo))

	buttons := fmt.Sprintf("%s  %s", yesButton, noButton)
	b.WriteString(buttons + "\n\n")

	// Help text
	help := m.theme.Subtle.Render("←/→: Navigate | Enter: Confirm | Esc: Cancel")
	b.WriteString(help)

	// Center and box it
	content := b.String()
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2).
		Width(60).
		Align(lipgloss.Center)

	box := boxStyle.Render(content)

	// Center vertically
	verticalPadding := (m.height - lipgloss.Height(box)) / 2
	if verticalPadding > 0 {
		box = strings.Repeat("\n", verticalPadding) + box
	}

	return box
}

// handleConfirmKey handles keyboard input in confirm dialog
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		m.confirmSelected = 0
		return m, nil
	case "right", "l":
		m.confirmSelected = 1
		return m, nil
	case "enter":
		confirmed := m.confirmSelected == 0
		callback := m.confirmCallback
		m.PopView()
		if callback != nil {
			return m, callback(confirmed)
		}
		return m, nil
	case "esc":
		m.PopView()
		return m, nil
	}
	return m, nil
}

// ============================================================================
// Phase 1: Progress Indicator
// ============================================================================

// ShowProgress displays a progress indicator
func (m *Model) ShowProgress(title string) {
	m.PushView(ViewModeProgress)
	m.progressTitle = title
	m.progressPercent = 0
	m.progressBytes = 0
	m.progressTotal = 0
	m.progressSpeed = ""
	m.progressETA = ""
	m.progressStatus = ""
	m.progressErr = nil
}

// UpdateProgress updates the progress indicator
func (m *Model) UpdateProgress(percent float64, bytes, total int64, speed, eta, status string) {
	m.progressPercent = percent
	m.progressBytes = bytes
	m.progressTotal = total
	m.progressSpeed = speed
	m.progressETA = eta
	m.progressStatus = status
}

// SetProgressError sets an error for the progress indicator
func (m *Model) SetProgressError(err error) {
	m.progressErr = err
}

// CloseProgress closes the progress indicator and returns to previous view
func (m *Model) CloseProgress() {
	m.PopView()
}

// viewProgress renders the progress indicator
func (m Model) viewProgress() string {
	var b strings.Builder

	// Title
	title := m.theme.Title.Render(m.progressTitle)
	b.WriteString(title + "\n\n")

	// Error display
	if m.progressErr != nil {
		errorMsg := m.theme.Error.Render(fmt.Sprintf("❌ Error: %s", m.progressErr.Error()))
		b.WriteString(errorMsg + "\n\n")

		help := m.theme.Subtle.Render("Enter: Close | Esc: Cancel")
		b.WriteString(help)
	} else {
		// Progress bar
		barWidth := 50
		filled := int(m.progressPercent * float64(barWidth) / 100)
		if filled > barWidth {
			filled = barWidth
		}

		bar := strings.Repeat("━", filled) + strings.Repeat("─", barWidth-filled)
		progressBar := fmt.Sprintf("[%s] %.1f%%", bar, m.progressPercent)
		b.WriteString(m.theme.Accent.Render(progressBar) + "\n\n")

		// Stats
		if m.progressTotal > 0 {
			downloaded := formatBytes(m.progressBytes)
			total := formatBytes(m.progressTotal)
			stats := fmt.Sprintf("Downloaded: %s / %s", downloaded, total)
			b.WriteString(m.theme.Normal.Render(stats) + "\n")
		}

		if m.progressSpeed != "" {
			speed := fmt.Sprintf("Speed: %s", m.progressSpeed)
			b.WriteString(m.theme.Normal.Render(speed) + "\n")
		}

		if m.progressETA != "" {
			eta := fmt.Sprintf("ETA: %s", m.progressETA)
			b.WriteString(m.theme.Normal.Render(eta) + "\n")
		}

		if m.progressStatus != "" {
			b.WriteString("\n" + m.theme.Subtle.Render(m.progressStatus) + "\n")
		}

		b.WriteString("\n")

		// Help text
		help := m.theme.Subtle.Render("Ctrl+C: Cancel")
		b.WriteString(help)
	}

	// Center and box it
	content := b.String()
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2).
		Width(70).
		Align(lipgloss.Left)

	box := boxStyle.Render(content)

	// Center vertically
	verticalPadding := (m.height - lipgloss.Height(box)) / 2
	if verticalPadding > 0 {
		box = strings.Repeat("\n", verticalPadding) + box
	}

	return box
}

// handleProgressKey handles keyboard input in progress view
func (m Model) handleProgressKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.progressErr != nil {
			m.PopView()
		}
		return m, nil
	case "esc", "ctrl+c":
		m.PopView()
		return m, tea.Quit
	}
	return m, nil
}

// ============================================================================
// Phase 1: Model Installation Functions
// ============================================================================

// isModelInstalled checks if a model is installed
func isModelInstalled(provider, modelID string) bool {
	if provider != "ollama" {
		// For cloud providers, models are always "available"
		return true
	}

	// For Ollama, check if model exists locally
	models := GetModelsForProvider("ollama")
	for _, m := range models {
		if m.ID == modelID && m.IsInstalled {
			return true
		}
	}
	return false
}

// installModelWithProgress installs a model with progress indicator
func (m *Model) installModelWithProgress(provider, modelID string) tea.Cmd {
	if provider != "ollama" {
		// Only Ollama models need installation
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("Model %s is ready to use.", modelID),
		})
		m.mode = ViewModeChat
		return nil
	}

	// Show progress view
	m.ShowProgress(fmt.Sprintf("Installing %s", modelID))

	return func() tea.Msg {
		return &ModelInstallStartMsg{
			Provider: provider,
			ModelID:  modelID,
		}
	}
}

// ModelInstallStartMsg signals to start model installation
type ModelInstallStartMsg struct {
	Provider string
	ModelID  string
}

// ModelInstallProgressMsg updates installation progress
type ModelInstallProgressMsg struct {
	Percent float64
	Bytes   int64
	Total   int64
	Speed   string
	ETA     string
	Status  string
}

// ModelInstallCompleteMsg signals installation completion
type ModelInstallCompleteMsg struct {
	ModelID string
	Success bool
	Error   error
}

// startModelInstallation starts model installation in background
func (m *Model) startModelInstallation(provider, modelID string) tea.Cmd {
	if provider != "ollama" {
		return func() tea.Msg {
			return &ModelInstallCompleteMsg{
				ModelID: modelID,
				Success: true,
			}
		}
	}

	return func() tea.Msg {
		// Start ollama pull in background
		cmd := exec.Command("ollama", "pull", modelID)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return &ModelInstallCompleteMsg{
				ModelID: modelID,
				Success: false,
				Error:   fmt.Errorf("failed to create pipe: %w", err),
			}
		}

		if err := cmd.Start(); err != nil {
			return &ModelInstallCompleteMsg{
				ModelID: modelID,
				Success: false,
				Error:   fmt.Errorf("failed to start ollama: %w", err),
			}
		}

		// Parse progress from stdout
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// Parse ollama progress output
			// Format: "pulling manifest" or "pulling <hash>... <percent>%"
			if strings.Contains(line, "%") {
				// Extract percentage
				parts := strings.Fields(line)
				for _, part := range parts {
					if strings.HasSuffix(part, "%") {
						percentStr := strings.TrimSuffix(part, "%")
						if percent, err := parseFloat(percentStr); err == nil {
							// Send progress update (this would need channel-based approach)
							_ = percent
						}
					}
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			return &ModelInstallCompleteMsg{
				ModelID: modelID,
				Success: false,
				Error:   fmt.Errorf("installation failed: %w", err),
			}
		}

		return &ModelInstallCompleteMsg{
			ModelID: modelID,
			Success: true,
		}
	}
}

// parseFloat safely parses a float string
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// ============================================================================
// Phase 2: Work Mode System (Ask/Edit/Agent/Plan)
// ============================================================================

// switchWorkMode switches to a different work mode
func (m *Model) switchWorkMode(mode WorkMode) tea.Cmd {
	m.workMode = mode

	// Add mode switch notification
	modeDesc := ""
	switch mode {
	case WorkModeAsk:
		modeDesc = "Quick questions and answers"
	case WorkModeEdit:
		modeDesc = "Code editing with file context"
		// Auto-detect files in current directory
		m.detectEditContext()
	case WorkModeAgent:
		modeDesc = "Autonomous multi-step task execution"
	case WorkModePlan:
		modeDesc = "Task analysis and planning"
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("%s Mode: %s\n\n%s", mode.Icon(), mode.String(), modeDesc),
	})

	return nil
}

// detectEditContext auto-detects files for Edit mode
func (m *Model) detectEditContext() {
	// Try to get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	// Look for common code files in current directory
	// This is a simple implementation - could be enhanced
	patterns := []string{"*.go", "*.py", "*.js", "*.ts", "*.java", "*.c", "*.cpp"}
	m.editContextFiles = []string{}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(cwd, pattern))
		m.editContextFiles = append(m.editContextFiles, matches...)
	}

	// Limit to first 10 files
	if len(m.editContextFiles) > 10 {
		m.editContextFiles = m.editContextFiles[:10]
	}
}

// buildPromptForMode builds a mode-specific prompt
func (m *Model) buildPromptForMode(userInput string) string {
	switch m.workMode {
	case WorkModeAsk:
		return m.buildAskPrompt(userInput)
	case WorkModeEdit:
		return m.buildEditPrompt(userInput)
	case WorkModeAgent:
		return m.buildAgentPrompt(userInput)
	case WorkModePlan:
		return m.buildPlanPrompt(userInput)
	default:
		return userInput
	}
}

// buildAskPrompt builds a prompt for Ask mode
func (m *Model) buildAskPrompt(userInput string) string {
	return fmt.Sprintf(`You are a helpful assistant. Answer the following question concisely and accurately.

Question: %s

Provide a clear, direct answer. Keep it brief unless more detail is specifically requested.`, userInput)
}

// buildEditPrompt builds a prompt for Edit mode
func (m *Model) buildEditPrompt(userInput string) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert code editor. Your task is to modify code based on the request.\n\n")

	// Add file context if available
	if len(m.editContextFiles) > 0 {
		prompt.WriteString("## File Context\n\n")
		for i, file := range m.editContextFiles {
			if i >= 3 {
				break // Limit to 3 files to avoid token overflow
			}
			content, err := os.ReadFile(file)
			if err == nil && len(content) < 10000 { // Limit file size
				prompt.WriteString(fmt.Sprintf("### File: %s\n```\n%s\n```\n\n", file, string(content)))
			}
		}
	}

	prompt.WriteString(fmt.Sprintf("## Task\n%s\n\n", userInput))
	prompt.WriteString("## Instructions\n")
	prompt.WriteString("1. Analyze the code and the requested changes\n")
	prompt.WriteString("2. Provide the modified code with clear explanations\n")
	prompt.WriteString("3. Highlight what changed and why\n")
	prompt.WriteString("4. Suggest any additional improvements if relevant\n")

	return prompt.String()
}

// buildAgentPrompt builds a prompt for Agent mode
func (m *Model) buildAgentPrompt(userInput string) string {
	return fmt.Sprintf(`You are an autonomous agent capable of breaking down complex tasks into steps and executing them.

Task: %s

Please:
1. Break down this task into 3-6 clear, executable steps
2. For each step, identify:
   - What needs to be done
   - What tools or resources are needed
   - Estimated time
3. Execute the steps systematically
4. Report progress and results for each step

Format your response as a structured plan first, then proceed with execution.`, userInput)
}

// buildPlanPrompt builds a prompt for Plan mode
func (m *Model) buildPlanPrompt(userInput string) string {
	return fmt.Sprintf(`You are a planning expert. Analyze this goal and create a detailed execution plan.

Goal: %s

Provide a comprehensive plan including:

1. **Step-by-Step Breakdown**: 
   - Number each step
   - Provide clear descriptions
   - Estimate time for each step (in minutes)

2. **Risk Assessment**:
   - Identify potential risks (Low/Medium/High)
   - Suggest mitigation strategies

3. **Dependencies**:
   - List required tools, libraries, or resources
   - Note any prerequisite steps

4. **Files Affected**:
   - Which files will be created/modified
   - Impact analysis

5. **Total Estimate**:
   - Overall time estimate
   - Complexity assessment

Format your response in clear sections with markdown.`, userInput)
}

// getWorkModePromptModifier returns a system message modifier based on work mode
func (m *Model) getWorkModePromptModifier() string {
	switch m.workMode {
	case WorkModeAsk:
		return "You are in Ask mode. Provide concise, accurate answers to user questions."
	case WorkModeEdit:
		return "You are in Edit mode. Focus on code analysis and modification. Provide clear diffs and explanations."
	case WorkModeAgent:
		return "You are in Agent mode. Break down tasks into steps and execute them systematically. Show progress."
	case WorkModePlan:
		return "You are in Plan mode. Analyze tasks and create detailed execution plans with risk assessment."
	default:
		return ""
	}
}

// ================== Phase 3: Multi-Model Functions ==================

// toggleMultiModelMode toggles multi-model mode on/off
func (m *Model) toggleMultiModelMode() tea.Cmd {
	m.multiModelEnabled = !m.multiModelEnabled

	status := "disabled"
	if m.multiModelEnabled {
		status = "enabled"
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("🔀 Multi-model mode %s\n\nUse /multimodel to select models for comparison.", status),
	})

	return nil
}

// initializeModelSelections populates available models for selection
func (m *Model) initializeModelSelections() {
	m.selectedModels = []ModelSelection{}

	// Get all available providers and their models
	providers := []string{"ollama", "anthropic", "openai", "google", "groq", "openrouter"}

	for _, provider := range providers {
		models := GetModelsForProvider(provider)
		for _, model := range models {
			// Only include installed models for Ollama, all models for cloud providers
			if provider == "ollama" && !model.IsInstalled {
				continue
			}

			displayName := fmt.Sprintf("%s - %s", provider, model.Name)
			if model.Description != "" {
				displayName = fmt.Sprintf("%s (%s)", displayName, model.Description)
			}

			selection := ModelSelection{
				Provider:    provider,
				Model:       model.ID,
				DisplayName: displayName,
				Selected:    false,
			}

			m.selectedModels = append(m.selectedModels, selection)
		}
	}
}

// sendToMultipleModels sends a message to all selected models in parallel
func (m *Model) sendToMultipleModels(text string) tea.Cmd {
	if !m.multiModelEnabled || len(m.selectedModels) == 0 {
		return m.sendToLLM(text)
	}

	// Count selected models
	selectedCount := 0
	for _, model := range m.selectedModels {
		if model.Selected {
			selectedCount++
		}
	}

	if selectedCount == 0 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "⚠️  No models selected. Use /multimodel to select models.",
		})
		return nil
	}

	// Clear previous responses
	m.modelResponses = make(map[string]*ModelResponse)
	m.showCompareView = true

	// Build mode-specific prompt
	enhancedPrompt := m.buildPromptForMode(text)

	// Convert messages to the right format
	var chatMsgs []Message
	for _, msg := range m.messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Welcome to DevOrch") {
			continue
		}
		chatMsgs = append(chatMsgs, msg)
	}
	chatMsgs = append(chatMsgs, Message{Role: "user", Content: enhancedPrompt})

	// Send to all selected models in parallel
	var cmds []tea.Cmd
	for _, model := range m.selectedModels {
		if !model.Selected {
			continue
		}

		modelKey := fmt.Sprintf("%s:%s", model.Provider, model.Model)
		m.modelResponses[modelKey] = &ModelResponse{
			Provider:    model.Provider,
			Model:       model.Model,
			DisplayName: model.DisplayName,
			InProgress:  true,
			StartTime:   time.Now(),
		}

		// Create command to send to this model
		cmd := m.sendToSingleModel(model.Provider, model.Model, modelKey, chatMsgs)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

// sendToSingleModel sends a message to a single model
func (m *Model) sendToSingleModel(provider, model, modelKey string, messages []Message) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		startTime := time.Now()
		response, err := ChatWithProvider(ctx, provider, model, messages)
		duration := time.Since(startTime).Milliseconds()

		return ModelResponseMsg{
			ModelKey: modelKey,
			Content:  response,
			Duration: duration,
			Error:    err,
		}
	}
}

// ModelResponseMsg is sent when a model completes its response
type ModelResponseMsg struct {
	ModelKey string
	Content  string
	Duration int64
	Error    error
}

// handleModelResponse handles a response from a model
func (m *Model) handleModelResponse(msg ModelResponseMsg) {
	if resp, ok := m.modelResponses[msg.ModelKey]; ok {
		resp.Content = msg.Content
		resp.Duration = msg.Duration
		resp.Error = msg.Error
		resp.InProgress = false

		// Count completed responses
		completedCount := 0
		totalCount := 0
		for _, r := range m.modelResponses {
			totalCount++
			if !r.InProgress {
				completedCount++
			}
		}

		// Update viewport content
		if completedCount == totalCount {
			// All responses complete, show comparison
			m.viewport.SetContent(m.renderCompareView())
		} else {
			// Still waiting for some responses
			m.viewport.SetContent(m.renderCompareView())
		}
		m.viewport.GotoBottom()
	}
}

// rateModelResponse allows user to rate a model's response
func (m *Model) rateModelResponse(modelKey string, rating int) {
	m.modelRatings[modelKey] = ResponseRating{
		ModelKey: modelKey,
		Rating:   rating,
	}

	// Update viewport to show rating
	m.viewport.SetContent(m.renderCompareView())
}

// viewMultiModelSelect renders the multi-model selection view
func (m Model) viewMultiModelSelect() string {
	var sb strings.Builder

	sb.WriteString(m.theme.Title.Render("Select Models for Comparison") + "\n\n")
	sb.WriteString(m.theme.Subtle.Render("Use Space to toggle, Enter to confirm, Esc to cancel") + "\n\n")

	// Show selection list
	for i, model := range m.selectedModels {
		cursor := " "
		if i == m.selectionIdx {
			cursor = ">"
		}

		checkbox := "[ ]"
		if model.Selected {
			checkbox = "[✓]"
		}

		if i == m.selectionIdx {
			sb.WriteString(m.theme.Selected.Render(fmt.Sprintf("%s %s %s\n", cursor, checkbox, model.DisplayName)))
		} else {
			sb.WriteString(fmt.Sprintf("%s %s %s\n", cursor, checkbox, model.DisplayName))
		}
	}

	// Show selection count
	selectedCount := 0
	for _, model := range m.selectedModels {
		if model.Selected {
			selectedCount++
		}
	}

	sb.WriteString("\n")
	sb.WriteString(m.theme.Accent.Render(fmt.Sprintf("Selected: %d models", selectedCount)) + "\n")

	return sb.String()
}

// viewCompare renders the comparison view for multi-model responses
func (m Model) viewCompare() string {
	return m.renderCompareView()
}

// renderCompareView renders the comparison view content
func (m Model) renderCompareView() string {
	var sb strings.Builder

	sb.WriteString(m.theme.Title.Render("Multi-Model Comparison") + "\n\n")

	if len(m.modelResponses) == 0 {
		sb.WriteString(m.theme.Subtle.Render("No responses yet. Send a message to compare models.") + "\n")
		return sb.String()
	}

	// Sort model keys for consistent display
	modelKeys := make([]string, 0, len(m.modelResponses))
	for key := range m.modelResponses {
		modelKeys = append(modelKeys, key)
	}

	// Display each model's response
	for i, key := range modelKeys {
		resp := m.modelResponses[key]

		// Header with model name and status
		header := fmt.Sprintf("━━━ %s ━━━", resp.DisplayName)
		sb.WriteString(m.theme.Accent.Render(header) + "\n")

		// Duration and status
		if resp.InProgress {
			elapsed := time.Since(resp.StartTime).Seconds()
			sb.WriteString(m.theme.Subtle.Render(fmt.Sprintf("⏳ In progress... (%.1fs elapsed)", elapsed)) + "\n\n")
		} else if resp.Error != nil {
			sb.WriteString(m.theme.Error.Render(fmt.Sprintf("❌ Error: %v", resp.Error)) + "\n")
			sb.WriteString(m.theme.Subtle.Render(fmt.Sprintf("Duration: %dms", resp.Duration)) + "\n\n")
		} else {
			sb.WriteString(m.theme.Success.Render(fmt.Sprintf("✓ Complete (%dms)", resp.Duration)) + "\n\n")

			// Response content
			sb.WriteString(resp.Content + "\n\n")

			// Rating
			if rating, ok := m.modelRatings[key]; ok {
				ratingIcon := ""
				if rating.Rating == 2 {
					ratingIcon = "👍"
				} else if rating.Rating == 1 {
					ratingIcon = "👎"
				}
				sb.WriteString(m.theme.Subtle.Render(fmt.Sprintf("Your rating: %s", ratingIcon)) + "\n")
			} else {
				sb.WriteString(m.theme.Subtle.Render("Press 1 for 👎, 2 for 👍 to rate this response") + "\n")
			}
		}

		if i < len(modelKeys)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
