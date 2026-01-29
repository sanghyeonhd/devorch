// Package lsp provides Language Server Protocol types.
package lsp

import "encoding/json"

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError represents a JSON-RPC error.
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Position in a text document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LocationLink represents a link between a source and a target location.
type LocationLink struct {
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`
	TargetURI            string `json:"targetUri"`
	TargetRange          Range  `json:"targetRange"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a text document.
type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"`
}

// TextDocumentItem represents an item to transfer a text document from the client to the server.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentPositionParams represents a position in a text document.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// InitializeParams for the initialize request.
type InitializeParams struct {
	ProcessID             *int               `json:"processId"`
	RootURI               string             `json:"rootUri"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions any                `json:"initializationOptions,omitempty"`
}

// InitializeResult from the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ClientCapabilities define capabilities provided by the client.
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

// TextDocumentClientCapabilities for text document features.
type TextDocumentClientCapabilities struct {
	Synchronization TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	Completion      CompletionClientCapabilities       `json:"completion,omitempty"`
	Hover           HoverClientCapabilities            `json:"hover,omitempty"`
	Definition      DefinitionClientCapabilities       `json:"definition,omitempty"`
	References      ReferencesClientCapabilities       `json:"references,omitempty"`
	DocumentSymbol  DocumentSymbolClientCapabilities   `json:"documentSymbol,omitempty"`
}

// TextDocumentSyncClientCapabilities for document synchronization.
type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

// CompletionClientCapabilities for completion.
type CompletionClientCapabilities struct {
	DynamicRegistration bool                       `json:"dynamicRegistration,omitempty"`
	CompletionItem      CompletionItemCapabilities `json:"completionItem,omitempty"`
}

// CompletionItemCapabilities for completion items.
type CompletionItemCapabilities struct {
	SnippetSupport      bool                          `json:"snippetSupport,omitempty"`
	DocumentationFormat []string                      `json:"documentationFormat,omitempty"`
	DeprecatedSupport   bool                          `json:"deprecatedSupport,omitempty"`
	ResolveSupport      *CompletionItemResolveSupport `json:"resolveSupport,omitempty"`
}

// CompletionItemResolveSupport capabilities.
type CompletionItemResolveSupport struct {
	Properties []string `json:"properties"`
}

// HoverClientCapabilities for hover.
type HoverClientCapabilities struct {
	DynamicRegistration bool     `json:"dynamicRegistration,omitempty"`
	ContentFormat       []string `json:"contentFormat,omitempty"`
}

// DefinitionClientCapabilities for go to definition.
type DefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// ReferencesClientCapabilities for find references.
type ReferencesClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// DocumentSymbolClientCapabilities for document symbols.
type DocumentSymbolClientCapabilities struct {
	DynamicRegistration               bool `json:"dynamicRegistration,omitempty"`
	HierarchicalDocumentSymbolSupport bool `json:"hierarchicalDocumentSymbolSupport,omitempty"`
}

// WorkspaceClientCapabilities for workspace features.
type WorkspaceClientCapabilities struct {
	ApplyEdit     bool                              `json:"applyEdit,omitempty"`
	WorkspaceEdit WorkspaceEditClientCapabilities   `json:"workspaceEdit,omitempty"`
	Symbol        WorkspaceSymbolClientCapabilities `json:"symbol,omitempty"`
}

// WorkspaceEditClientCapabilities for workspace edit.
type WorkspaceEditClientCapabilities struct {
	DocumentChanges bool `json:"documentChanges,omitempty"`
}

// WorkspaceSymbolClientCapabilities for workspace symbols.
type WorkspaceSymbolClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// ServerCapabilities define capabilities provided by the server.
type ServerCapabilities struct {
	TextDocumentSync           *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	CompletionProvider         *CompletionOptions       `json:"completionProvider,omitempty"`
	HoverProvider              bool                     `json:"hoverProvider,omitempty"`
	DefinitionProvider         bool                     `json:"definitionProvider,omitempty"`
	ReferencesProvider         bool                     `json:"referencesProvider,omitempty"`
	DocumentSymbolProvider     bool                     `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider    bool                     `json:"workspaceSymbolProvider,omitempty"`
	CodeActionProvider         bool                     `json:"codeActionProvider,omitempty"`
	DocumentFormattingProvider bool                     `json:"documentFormattingProvider,omitempty"`
	RenameProvider             bool                     `json:"renameProvider,omitempty"`
	DiagnosticProvider         *DiagnosticOptions       `json:"diagnosticProvider,omitempty"`
}

// TextDocumentSyncOptions for document synchronization.
type TextDocumentSyncOptions struct {
	OpenClose bool         `json:"openClose,omitempty"`
	Change    int          `json:"change,omitempty"` // 0: None, 1: Full, 2: Incremental
	Save      *SaveOptions `json:"save,omitempty"`
}

// SaveOptions for text document save.
type SaveOptions struct {
	IncludeText bool `json:"includeText,omitempty"`
}

// CompletionOptions for completion.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
}

// DiagnosticOptions for diagnostics.
type DiagnosticOptions struct {
	Identifier            string `json:"identifier,omitempty"`
	InterFileDependencies bool   `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool   `json:"workspaceDiagnostics"`
}

// Hover represents hover information.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// MarkupContent represents text content with a kind.
type MarkupContent struct {
	Kind  string `json:"kind"` // "plaintext" or "markdown"
	Value string `json:"value"`
}

// CompletionList represents a collection of completion items.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// CompletionItem represents a completion item.
type CompletionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind,omitempty"`
	Detail        string         `json:"detail,omitempty"`
	Documentation *MarkupContent `json:"documentation,omitempty"`
	InsertText    string         `json:"insertText,omitempty"`
	TextEdit      *TextEdit      `json:"textEdit,omitempty"`
}

// TextEdit represents a text edit.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// Diagnostic represents a diagnostic, such as an error or warning.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"` // 1: Error, 2: Warning, 3: Info, 4: Hint
	Code     any    `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// DocumentSymbol represents programming constructs like variables, classes, etc.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation represents information about programming constructs.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// Symbol kinds
const (
	SymbolKindFile          = 1
	SymbolKindModule        = 2
	SymbolKindNamespace     = 3
	SymbolKindPackage       = 4
	SymbolKindClass         = 5
	SymbolKindMethod        = 6
	SymbolKindProperty      = 7
	SymbolKindField         = 8
	SymbolKindConstructor   = 9
	SymbolKindEnum          = 10
	SymbolKindInterface     = 11
	SymbolKindFunction      = 12
	SymbolKindVariable      = 13
	SymbolKindConstant      = 14
	SymbolKindString        = 15
	SymbolKindNumber        = 16
	SymbolKindBoolean       = 17
	SymbolKindArray         = 18
	SymbolKindObject        = 19
	SymbolKindKey           = 20
	SymbolKindNull          = 21
	SymbolKindEnumMember    = 22
	SymbolKindStruct        = 23
	SymbolKindEvent         = 24
	SymbolKindOperator      = 25
	SymbolKindTypeParameter = 26
)

// Completion item kinds
const (
	CompletionItemKindText          = 1
	CompletionItemKindMethod        = 2
	CompletionItemKindFunction      = 3
	CompletionItemKindConstructor   = 4
	CompletionItemKindField         = 5
	CompletionItemKindVariable      = 6
	CompletionItemKindClass         = 7
	CompletionItemKindInterface     = 8
	CompletionItemKindModule        = 9
	CompletionItemKindProperty      = 10
	CompletionItemKindUnit          = 11
	CompletionItemKindValue         = 12
	CompletionItemKindEnum          = 13
	CompletionItemKindKeyword       = 14
	CompletionItemKindSnippet       = 15
	CompletionItemKindColor         = 16
	CompletionItemKindFile          = 17
	CompletionItemKindReference     = 18
	CompletionItemKindFolder        = 19
	CompletionItemKindEnumMember    = 20
	CompletionItemKindConstant      = 21
	CompletionItemKindStruct        = 22
	CompletionItemKindEvent         = 23
	CompletionItemKindOperator      = 24
	CompletionItemKindTypeParameter = 25
)

// Diagnostic severity
const (
	DiagnosticSeverityError   = 1
	DiagnosticSeverityWarning = 2
	DiagnosticSeverityInfo    = 3
	DiagnosticSeverityHint    = 4
)

// CallHierarchyItem represents a call hierarchy item.
type CallHierarchyItem struct {
	Name           string `json:"name"`
	Kind           int    `json:"kind"`
	Tags           []int  `json:"tags,omitempty"`
	Detail         string `json:"detail,omitempty"`
	URI            string `json:"uri"`
	Range          Range  `json:"range"`
	SelectionRange Range  `json:"selectionRange"`
	Data           any    `json:"data,omitempty"`
}

// CallHierarchyIncomingCall represents an incoming call.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// CallHierarchyOutgoingCall represents an outgoing call.
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}
