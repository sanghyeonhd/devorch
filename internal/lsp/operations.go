// Package lsp provides Language Server Protocol operations.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
)

// TextDocumentDidOpen notifies the server that a document was opened.
func (c *Client) TextDocumentDidOpen(ctx context.Context, uri, languageID, text string, version int) error {
	params := struct {
		TextDocument TextDocumentItem `json:"textDocument"`
	}{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    version,
			Text:       text,
		},
	}
	return c.notify("textDocument/didOpen", params)
}

// TextDocumentDidClose notifies the server that a document was closed.
func (c *Client) TextDocumentDidClose(ctx context.Context, uri string) error {
	params := struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}
	return c.notify("textDocument/didClose", params)
}

// TextDocumentDidChange notifies the server that a document changed.
func (c *Client) TextDocumentDidChange(ctx context.Context, uri string, version int, text string) error {
	params := struct {
		TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}{
		TextDocument: VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: TextDocumentIdentifier{URI: uri},
			Version:                version,
		},
		ContentChanges: []struct {
			Text string `json:"text"`
		}{{Text: text}},
	}
	return c.notify("textDocument/didChange", params)
}

// GotoDefinition requests the definition of a symbol.
func (c *Client) GotoDefinition(ctx context.Context, uri string, line, character int) ([]Location, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	resp, err := c.request(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	// Response can be Location, []Location, or []LocationLink
	var locations []Location

	// Try as single Location
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err == nil && loc.URI != "" {
		return []Location{loc}, nil
	}

	// Try as []Location
	if err := json.Unmarshal(resp.Result, &locations); err == nil {
		return locations, nil
	}

	// Try as []LocationLink
	var links []LocationLink
	if err := json.Unmarshal(resp.Result, &links); err == nil {
		locations = make([]Location, len(links))
		for i, link := range links {
			locations[i] = Location{
				URI:   link.TargetURI,
				Range: link.TargetRange,
			}
		}
		return locations, nil
	}

	return nil, fmt.Errorf("failed to parse definition response")
}

// FindReferences finds all references to a symbol.
func (c *Client) FindReferences(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]Location, error) {
	params := struct {
		TextDocumentPositionParams
		Context struct {
			IncludeDeclaration bool `json:"includeDeclaration"`
		} `json:"context"`
	}{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: character},
		},
	}
	params.Context.IncludeDeclaration = includeDeclaration

	resp, err := c.request(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(resp.Result, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse references response: %w", err)
	}

	return locations, nil
}

// Hover returns hover information for a position.
func (c *Client) Hover(ctx context.Context, uri string, line, character int) (*Hover, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	resp, err := c.request(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	if string(resp.Result) == "null" {
		return nil, nil
	}

	var hover Hover
	if err := json.Unmarshal(resp.Result, &hover); err != nil {
		return nil, fmt.Errorf("failed to parse hover response: %w", err)
	}

	return &hover, nil
}

// Completion requests completion items at a position.
func (c *Client) Completion(ctx context.Context, uri string, line, character int) (*CompletionList, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	resp, err := c.request(ctx, "textDocument/completion", params)
	if err != nil {
		return nil, err
	}

	// Response can be []CompletionItem or CompletionList
	var list CompletionList
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		// Try as []CompletionItem
		var items []CompletionItem
		if err := json.Unmarshal(resp.Result, &items); err != nil {
			return nil, fmt.Errorf("failed to parse completion response: %w", err)
		}
		list.Items = items
	}

	return &list, nil
}

// DocumentSymbols returns symbols in a document.
func (c *Client) DocumentSymbols(ctx context.Context, uri string) ([]DocumentSymbol, error) {
	params := struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	resp, err := c.request(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	// Response can be []DocumentSymbol or []SymbolInformation
	var symbols []DocumentSymbol
	if err := json.Unmarshal(resp.Result, &symbols); err != nil {
		// Try as []SymbolInformation and convert
		var infos []SymbolInformation
		if err := json.Unmarshal(resp.Result, &infos); err != nil {
			return nil, fmt.Errorf("failed to parse document symbols: %w", err)
		}

		symbols = make([]DocumentSymbol, len(infos))
		for i, info := range infos {
			symbols[i] = DocumentSymbol{
				Name:           info.Name,
				Kind:           info.Kind,
				Range:          info.Location.Range,
				SelectionRange: info.Location.Range,
			}
		}
	}

	return symbols, nil
}

// WorkspaceSymbols searches for symbols in the workspace.
func (c *Client) WorkspaceSymbols(ctx context.Context, query string) ([]SymbolInformation, error) {
	params := struct {
		Query string `json:"query"`
	}{
		Query: query,
	}

	resp, err := c.request(ctx, "workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []SymbolInformation
	if err := json.Unmarshal(resp.Result, &symbols); err != nil {
		return nil, fmt.Errorf("failed to parse workspace symbols: %w", err)
	}

	return symbols, nil
}

// Formatting requests document formatting.
func (c *Client) Formatting(ctx context.Context, uri string, tabSize int, insertSpaces bool) ([]TextEdit, error) {
	params := struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
		Options      struct {
			TabSize      int  `json:"tabSize"`
			InsertSpaces bool `json:"insertSpaces"`
		} `json:"options"`
	}{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}
	params.Options.TabSize = tabSize
	params.Options.InsertSpaces = insertSpaces

	resp, err := c.request(ctx, "textDocument/formatting", params)
	if err != nil {
		return nil, err
	}

	var edits []TextEdit
	if err := json.Unmarshal(resp.Result, &edits); err != nil {
		return nil, fmt.Errorf("failed to parse formatting response: %w", err)
	}

	return edits, nil
}

// Rename renames a symbol.
func (c *Client) Rename(ctx context.Context, uri string, line, character int, newName string) (map[string][]TextEdit, error) {
	params := struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
		Position     Position               `json:"position"`
		NewName      string                 `json:"newName"`
	}{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
		NewName:      newName,
	}

	resp, err := c.request(ctx, "textDocument/rename", params)
	if err != nil {
		return nil, err
	}

	var workspaceEdit struct {
		Changes map[string][]TextEdit `json:"changes"`
	}
	if err := json.Unmarshal(resp.Result, &workspaceEdit); err != nil {
		return nil, fmt.Errorf("failed to parse rename response: %w", err)
	}

	return workspaceEdit.Changes, nil
}

// CodeAction requests code actions for a range.
func (c *Client) CodeAction(ctx context.Context, uri string, rng Range, diagnostics []Diagnostic) ([]CodeAction, error) {
	params := struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
		Range        Range                  `json:"range"`
		Context      struct {
			Diagnostics []Diagnostic `json:"diagnostics"`
		} `json:"context"`
	}{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        rng,
	}
	params.Context.Diagnostics = diagnostics

	resp, err := c.request(ctx, "textDocument/codeAction", params)
	if err != nil {
		return nil, err
	}

	var actions []CodeAction
	if err := json.Unmarshal(resp.Result, &actions); err != nil {
		return nil, fmt.Errorf("failed to parse code actions: %w", err)
	}

	return actions, nil
}

// CodeAction represents a code action.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	Command     *Command       `json:"command,omitempty"`
}

// WorkspaceEdit represents changes to many resources.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// Command represents a command.
type Command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

// GotoImplementation requests the implementation of a symbol.
func (c *Client) GotoImplementation(ctx context.Context, uri string, line, character int) ([]Location, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	resp, err := c.request(ctx, "textDocument/implementation", params)
	if err != nil {
		return nil, err
	}

	// Response can be Location, []Location, or []LocationLink
	var locations []Location

	// Try as single Location
	var loc Location
	if err := json.Unmarshal(resp.Result, &loc); err == nil && loc.URI != "" {
		return []Location{loc}, nil
	}

	// Try as []Location
	if err := json.Unmarshal(resp.Result, &locations); err == nil {
		return locations, nil
	}

	// Try as []LocationLink
	var links []LocationLink
	if err := json.Unmarshal(resp.Result, &links); err == nil {
		locations = make([]Location, len(links))
		for i, link := range links {
			locations[i] = Location{
				URI:   link.TargetURI,
				Range: link.TargetRange,
			}
		}
		return locations, nil
	}

	return nil, fmt.Errorf("failed to parse implementation response")
}

// PrepareCallHierarchy prepares call hierarchy at a position.
func (c *Client) PrepareCallHierarchy(ctx context.Context, uri string, line, character int) ([]CallHierarchyItem, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	resp, err := c.request(ctx, "textDocument/prepareCallHierarchy", params)
	if err != nil {
		return nil, err
	}

	if string(resp.Result) == "null" {
		return nil, nil
	}

	var items []CallHierarchyItem
	if err := json.Unmarshal(resp.Result, &items); err != nil {
		return nil, fmt.Errorf("failed to parse call hierarchy response: %w", err)
	}

	return items, nil
}

// IncomingCalls returns incoming calls for a call hierarchy item.
func (c *Client) IncomingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyIncomingCall, error) {
	params := struct {
		Item CallHierarchyItem `json:"item"`
	}{
		Item: item,
	}

	resp, err := c.request(ctx, "callHierarchy/incomingCalls", params)
	if err != nil {
		return nil, err
	}

	if string(resp.Result) == "null" {
		return nil, nil
	}

	var calls []CallHierarchyIncomingCall
	if err := json.Unmarshal(resp.Result, &calls); err != nil {
		return nil, fmt.Errorf("failed to parse incoming calls response: %w", err)
	}

	return calls, nil
}

// OutgoingCalls returns outgoing calls for a call hierarchy item.
func (c *Client) OutgoingCalls(ctx context.Context, item CallHierarchyItem) ([]CallHierarchyOutgoingCall, error) {
	params := struct {
		Item CallHierarchyItem `json:"item"`
	}{
		Item: item,
	}

	resp, err := c.request(ctx, "callHierarchy/outgoingCalls", params)
	if err != nil {
		return nil, err
	}

	if string(resp.Result) == "null" {
		return nil, nil
	}

	var calls []CallHierarchyOutgoingCall
	if err := json.Unmarshal(resp.Result, &calls); err != nil {
		return nil, fmt.Errorf("failed to parse outgoing calls response: %w", err)
	}

	return calls, nil
}
