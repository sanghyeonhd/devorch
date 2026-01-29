package extension

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Initialize(t *testing.T) {
	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "devorch-ext-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	loader := &Loader{
		baseDir:    tmpDir,
		extensions: make(map[string]*Extension),
	}

	if err := loader.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Check directories were created
	dirs := []string{"tools", "agents", "config"}
	for _, dir := range dirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}

	// Check example files were created
	exampleJSON := filepath.Join(tmpDir, "tools", "example.json")
	if _, err := os.Stat(exampleJSON); os.IsNotExist(err) {
		t.Error("Example JSON manifest was not created")
	}

	exampleScript := filepath.Join(tmpDir, "tools", "example.sh")
	if _, err := os.Stat(exampleScript); os.IsNotExist(err) {
		t.Error("Example script was not created")
	}
}

func TestLoader_LoadAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "devorch-ext-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directories
	toolsDir := filepath.Join(tmpDir, "tools")
	agentsDir := filepath.Join(tmpDir, "agents")
	os.MkdirAll(toolsDir, 0755)
	os.MkdirAll(agentsDir, 0755)

	// Create a test tool script
	scriptPath := filepath.Join(toolsDir, "test-tool.sh")
	script := `#!/bin/bash
echo "test output"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Create a test tool manifest
	ext := &Extension{
		Name:        "test-tool",
		Description: "A test tool",
		Path:        scriptPath,
		Command:     "bash",
		Parameters: []ParameterDef{
			{Name: "input", Type: "string", Required: true},
		},
	}

	manifestPath := filepath.Join(toolsDir, "test-tool.json")
	data, _ := json.Marshal(ext)
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	loader := &Loader{
		baseDir:    tmpDir,
		extensions: make(map[string]*Extension),
	}

	if err := loader.LoadAll(); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	// Verify extension was loaded
	if len(loader.extensions) != 1 {
		t.Errorf("Expected 1 extension, got %d", len(loader.extensions))
	}

	loaded, ok := loader.GetExtension("test-tool")
	if !ok {
		t.Error("test-tool extension not found")
	}

	if loaded.Name != "test-tool" {
		t.Errorf("Expected name 'test-tool', got '%s'", loaded.Name)
	}
}

func TestLoader_Execute(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "devorch-ext-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	toolsDir := filepath.Join(tmpDir, "tools")
	os.MkdirAll(toolsDir, 0755)

	// Create a test script that echoes input
	scriptPath := filepath.Join(toolsDir, "echo-tool.sh")
	script := `#!/bin/bash
cat | jq -r '.message'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	ext := &Extension{
		Name:        "echo-tool",
		Description: "Echoes input",
		Path:        scriptPath,
		Command:     "bash",
		Parameters: []ParameterDef{
			{Name: "message", Type: "string", Required: true},
		},
	}

	loader := &Loader{
		baseDir: tmpDir,
		extensions: map[string]*Extension{
			"echo-tool": ext,
		},
	}

	// Skip this test if jq is not installed
	if _, err := os.Stat("/usr/bin/jq"); os.IsNotExist(err) {
		if _, err := os.Stat("/opt/homebrew/bin/jq"); os.IsNotExist(err) {
			t.Skip("jq not installed, skipping execution test")
		}
	}

	ctx := context.Background()
	output, err := loader.Execute(ctx, "echo-tool", map[string]any{
		"message": "hello world",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := "hello world\n"
	if output != expected {
		t.Errorf("Expected output '%s', got '%s'", expected, output)
	}
}

func TestExtensionTool_Info(t *testing.T) {
	ext := &Extension{
		Name:        "my-tool",
		Description: "My custom tool",
		Parameters: []ParameterDef{
			{Name: "input", Type: "string", Description: "Input text", Required: true},
			{Name: "verbose", Type: "boolean", Description: "Verbose output", Required: false},
		},
	}

	loader := &Loader{
		extensions: map[string]*Extension{
			"my-tool": ext,
		},
	}

	tool := NewExtensionTool(loader, ext)
	info := tool.Info()

	if info.Name != "ext_my-tool" {
		t.Errorf("Expected name 'ext_my-tool', got '%s'", info.Name)
	}

	// Parse parameters JSON schema
	var schema map[string]any
	if err := json.Unmarshal(info.Parameters, &schema); err != nil {
		t.Fatalf("Failed to parse parameters schema: %v", err)
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties in schema")
	}

	if len(props) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(props))
	}

	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatal("Expected required in schema")
	}

	if len(required) != 1 {
		t.Errorf("Expected 1 required parameter, got %d", len(required))
	}

	if required[0] != "input" {
		t.Errorf("Expected required parameter 'input', got '%v'", required[0])
	}
}

func TestLoader_ListExtensions(t *testing.T) {
	loader := &Loader{
		extensions: map[string]*Extension{
			"tool1":  {Name: "tool1", Type: "tool"},
			"tool2":  {Name: "tool2", Type: "tool"},
			"agent1": {Name: "agent1", Type: "agent"},
		},
	}

	all := loader.ListExtensions()
	if len(all) != 3 {
		t.Errorf("Expected 3 extensions, got %d", len(all))
	}

	tools := loader.ListTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	agents := loader.ListAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}
}
