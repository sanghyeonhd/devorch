package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/lsp"
	"devorch/internal/tool"
)

// LSPParams defines parameters for the LSP tool
type LSPParams struct {
	Operation string `json:"operation"` // goToDefinition, findReferences, hover, documentSymbol, workspaceSymbol, goToImplementation, prepareCallHierarchy, incomingCalls, outgoingCalls
	FilePath  string `json:"file_path"` // Absolute or relative path to the file
	Line      int    `json:"line"`      // Line number (1-based)
	Character int    `json:"character"` // Character offset (1-based)
	Query     string `json:"query"`     // For workspaceSymbol operation
}

// LSPTool provides Language Server Protocol operations
type LSPTool struct {
	manager *lsp.Manager
	rootDir string
}

// NewLSPTool creates a new LSP tool
func NewLSPTool(rootDir string) *LSPTool {
	return &LSPTool{
		manager: lsp.NewManager(),
		rootDir: rootDir,
	}
}

// Info returns tool metadata
func (t *LSPTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "lsp",
		Description: `Perform Language Server Protocol operations for code intelligence.

Available operations:
- goToDefinition: Jump to the definition of a symbol
- findReferences: Find all references to a symbol
- hover: Get hover information (type info, documentation)
- documentSymbol: List all symbols in the current document
- workspaceSymbol: Search for symbols across the workspace
- goToImplementation: Find implementations of an interface/method
- prepareCallHierarchy: Prepare call hierarchy at position
- incomingCalls: Find callers of a function
- outgoingCalls: Find functions called by a function

Examples:
  - {"operation": "goToDefinition", "file_path": "main.go", "line": 10, "character": 15}
  - {"operation": "findReferences", "file_path": "src/utils.ts", "line": 25, "character": 8}
  - {"operation": "hover", "file_path": "lib.py", "line": 42, "character": 12}
  - {"operation": "documentSymbol", "file_path": "module.rs"}
  - {"operation": "workspaceSymbol", "query": "handleRequest"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"operation": {
					"type": "string",
					"enum": ["goToDefinition", "findReferences", "hover", "documentSymbol", "workspaceSymbol", "goToImplementation", "prepareCallHierarchy", "incomingCalls", "outgoingCalls"],
					"description": "The LSP operation to perform"
				},
				"file_path": {
					"type": "string",
					"description": "The absolute or relative path to the file"
				},
				"line": {
					"type": "integer",
					"description": "The line number (1-based, as shown in editors)"
				},
				"character": {
					"type": "integer",
					"description": "The character offset (1-based, as shown in editors)"
				},
				"query": {
					"type": "string",
					"description": "Search query for workspaceSymbol operation"
				}
			},
			"required": ["operation"]
		}`),
	}
}

// Execute runs the LSP operation
func (t *LSPTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p LSPParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	// Resolve file path
	filePath := p.FilePath
	if filePath != "" && !filepath.IsAbs(filePath) {
		filePath = filepath.Join(t.rootDir, filePath)
	}

	// Check if file exists (for file-based operations)
	if filePath != "" && p.Operation != "workspaceSymbol" {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", filePath)
		}
	}

	// Get or create LSP client for this file
	client, err := t.getClient(ctx.Ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("no LSP server available for this file type: %w", err)
	}

	// Convert to URI
	uri := "file://" + filePath

	// Convert 1-based to 0-based positions
	line := p.Line - 1
	character := p.Character - 1
	if line < 0 {
		line = 0
	}
	if character < 0 {
		character = 0
	}

	// Generate title
	relPath := filePath
	if strings.HasPrefix(filePath, t.rootDir) {
		relPath, _ = filepath.Rel(t.rootDir, filePath)
	}
	title := fmt.Sprintf("%s %s:%d:%d", p.Operation, relPath, p.Line, p.Character)

	// Execute operation
	var result any
	var output string

	switch p.Operation {
	case "goToDefinition":
		locations, err := client.GotoDefinition(ctx.Ctx, uri, line, character)
		if err != nil {
			return nil, fmt.Errorf("goToDefinition failed: %w", err)
		}
		result = locations
		output = t.formatLocations(locations, "Definition")

	case "findReferences":
		locations, err := client.FindReferences(ctx.Ctx, uri, line, character, true)
		if err != nil {
			return nil, fmt.Errorf("findReferences failed: %w", err)
		}
		result = locations
		output = t.formatLocations(locations, "References")

	case "hover":
		hover, err := client.Hover(ctx.Ctx, uri, line, character)
		if err != nil {
			return nil, fmt.Errorf("hover failed: %w", err)
		}
		if hover == nil {
			output = "No hover information available"
		} else {
			result = hover
			output = t.formatHover(hover)
		}

	case "documentSymbol":
		symbols, err := client.DocumentSymbols(ctx.Ctx, uri)
		if err != nil {
			return nil, fmt.Errorf("documentSymbol failed: %w", err)
		}
		result = symbols
		output = t.formatDocumentSymbols(symbols)
		title = fmt.Sprintf("documentSymbol %s", relPath)

	case "workspaceSymbol":
		query := p.Query
		if query == "" {
			query = ""
		}
		symbols, err := client.WorkspaceSymbols(ctx.Ctx, query)
		if err != nil {
			return nil, fmt.Errorf("workspaceSymbol failed: %w", err)
		}
		result = symbols
		output = t.formatWorkspaceSymbols(symbols)
		title = fmt.Sprintf("workspaceSymbol %q", query)

	case "goToImplementation":
		locations, err := client.GotoImplementation(ctx.Ctx, uri, line, character)
		if err != nil {
			return nil, fmt.Errorf("goToImplementation failed: %w", err)
		}
		result = locations
		output = t.formatLocations(locations, "Implementations")

	case "prepareCallHierarchy":
		items, err := client.PrepareCallHierarchy(ctx.Ctx, uri, line, character)
		if err != nil {
			return nil, fmt.Errorf("prepareCallHierarchy failed: %w", err)
		}
		result = items
		output = t.formatCallHierarchyItems(items)

	case "incomingCalls":
		items, err := client.PrepareCallHierarchy(ctx.Ctx, uri, line, character)
		if err != nil {
			return nil, fmt.Errorf("prepareCallHierarchy failed: %w", err)
		}
		if len(items) == 0 {
			output = "No call hierarchy item at this position"
		} else {
			calls, err := client.IncomingCalls(ctx.Ctx, items[0])
			if err != nil {
				return nil, fmt.Errorf("incomingCalls failed: %w", err)
			}
			result = calls
			output = t.formatIncomingCalls(calls)
		}

	case "outgoingCalls":
		items, err := client.PrepareCallHierarchy(ctx.Ctx, uri, line, character)
		if err != nil {
			return nil, fmt.Errorf("prepareCallHierarchy failed: %w", err)
		}
		if len(items) == 0 {
			output = "No call hierarchy item at this position"
		} else {
			calls, err := client.OutgoingCalls(ctx.Ctx, items[0])
			if err != nil {
				return nil, fmt.Errorf("outgoingCalls failed: %w", err)
			}
			result = calls
			output = t.formatOutgoingCalls(calls)
		}

	default:
		return nil, fmt.Errorf("unknown operation: %s", p.Operation)
	}

	return &tool.Result{
		Title:  title,
		Output: output,
		Metadata: map[string]any{
			"operation": p.Operation,
			"result":    result,
		},
	}, nil
}

// getClient returns an LSP client for the given file
func (t *LSPTool) getClient(ctx context.Context, filePath string) (*lsp.Client, error) {
	// Detect language from file extension
	language := lsp.GetLanguageForFile(filePath)
	if language == "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		return nil, fmt.Errorf("no LSP server configured for extension: %s", ext)
	}

	// Get root URI
	rootURI := "file://" + t.rootDir

	// Get or create client
	client, err := t.manager.GetClient(ctx, language, rootURI)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Formatting helpers

func (t *LSPTool) formatLocations(locations []lsp.Location, label string) string {
	if len(locations) == 0 {
		return fmt.Sprintf("No %s found", strings.ToLower(label))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%d found):\n", label, len(locations)))

	for i, loc := range locations {
		path := strings.TrimPrefix(loc.URI, "file://")
		if strings.HasPrefix(path, t.rootDir) {
			path, _ = filepath.Rel(t.rootDir, path)
		}
		sb.WriteString(fmt.Sprintf("  %d. %s:%d:%d\n",
			i+1, path, loc.Range.Start.Line+1, loc.Range.Start.Character+1))
	}

	return sb.String()
}

func (t *LSPTool) formatHover(hover *lsp.Hover) string {
	if hover == nil {
		return "No hover information"
	}

	content := hover.Contents.Value
	if content == "" {
		return "No hover content"
	}

	return fmt.Sprintf("Hover Information:\n%s", content)
}

func (t *LSPTool) formatDocumentSymbols(symbols []lsp.DocumentSymbol) string {
	if len(symbols) == 0 {
		return "No symbols found in document"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Document Symbols (%d found):\n", len(symbols)))

	var formatSymbol func(s lsp.DocumentSymbol, indent string)
	formatSymbol = func(s lsp.DocumentSymbol, indent string) {
		sb.WriteString(fmt.Sprintf("%s• %s (%s) - line %d\n",
			indent, s.Name, symbolKindName(s.Kind), s.Range.Start.Line+1))
		for _, child := range s.Children {
			formatSymbol(child, indent+"  ")
		}
	}

	for _, s := range symbols {
		formatSymbol(s, "  ")
	}

	return sb.String()
}

func (t *LSPTool) formatWorkspaceSymbols(symbols []lsp.SymbolInformation) string {
	if len(symbols) == 0 {
		return "No workspace symbols found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Workspace Symbols (%d found):\n", len(symbols)))

	for i, s := range symbols {
		path := strings.TrimPrefix(s.Location.URI, "file://")
		if strings.HasPrefix(path, t.rootDir) {
			path, _ = filepath.Rel(t.rootDir, path)
		}
		sb.WriteString(fmt.Sprintf("  %d. %s (%s) - %s:%d\n",
			i+1, s.Name, symbolKindName(s.Kind), path, s.Location.Range.Start.Line+1))
	}

	return sb.String()
}

func (t *LSPTool) formatCallHierarchyItems(items []lsp.CallHierarchyItem) string {
	if len(items) == 0 {
		return "No call hierarchy items found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Call Hierarchy Items (%d found):\n", len(items)))

	for i, item := range items {
		path := strings.TrimPrefix(item.URI, "file://")
		if strings.HasPrefix(path, t.rootDir) {
			path, _ = filepath.Rel(t.rootDir, path)
		}
		sb.WriteString(fmt.Sprintf("  %d. %s (%s) - %s:%d\n",
			i+1, item.Name, symbolKindName(item.Kind), path, item.Range.Start.Line+1))
	}

	return sb.String()
}

func (t *LSPTool) formatIncomingCalls(calls []lsp.CallHierarchyIncomingCall) string {
	if len(calls) == 0 {
		return "No incoming calls found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Incoming Calls (%d found):\n", len(calls)))

	for i, call := range calls {
		path := strings.TrimPrefix(call.From.URI, "file://")
		if strings.HasPrefix(path, t.rootDir) {
			path, _ = filepath.Rel(t.rootDir, path)
		}
		sb.WriteString(fmt.Sprintf("  %d. %s (%s) - %s:%d\n",
			i+1, call.From.Name, symbolKindName(call.From.Kind), path, call.From.Range.Start.Line+1))
	}

	return sb.String()
}

func (t *LSPTool) formatOutgoingCalls(calls []lsp.CallHierarchyOutgoingCall) string {
	if len(calls) == 0 {
		return "No outgoing calls found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Outgoing Calls (%d found):\n", len(calls)))

	for i, call := range calls {
		path := strings.TrimPrefix(call.To.URI, "file://")
		if strings.HasPrefix(path, t.rootDir) {
			path, _ = filepath.Rel(t.rootDir, path)
		}
		sb.WriteString(fmt.Sprintf("  %d. %s (%s) - %s:%d\n",
			i+1, call.To.Name, symbolKindName(call.To.Kind), path, call.To.Range.Start.Line+1))
	}

	return sb.String()
}

// symbolKindName returns a human-readable name for a symbol kind
func symbolKindName(kind int) string {
	kinds := map[int]string{
		1:  "File",
		2:  "Module",
		3:  "Namespace",
		4:  "Package",
		5:  "Class",
		6:  "Method",
		7:  "Property",
		8:  "Field",
		9:  "Constructor",
		10: "Enum",
		11: "Interface",
		12: "Function",
		13: "Variable",
		14: "Constant",
		15: "String",
		16: "Number",
		17: "Boolean",
		18: "Array",
		19: "Object",
		20: "Key",
		21: "Null",
		22: "EnumMember",
		23: "Struct",
		24: "Event",
		25: "Operator",
		26: "TypeParameter",
	}
	if name, ok := kinds[kind]; ok {
		return name
	}
	return fmt.Sprintf("Kind(%d)", kind)
}
