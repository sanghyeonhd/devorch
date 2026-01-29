package mcp

import (
	"testing"
)

func TestGetRecommendedServers(t *testing.T) {
	servers := GetRecommendedServers()
	if len(servers) == 0 {
		t.Error("Expected at least one recommended server")
	}
	for _, s := range servers {
		if s.Name == "" {
			t.Error("Server name cannot be empty")
		}
		if s.Description == "" {
			t.Errorf("Server %s missing description", s.Name)
		}
		if s.Category == "" {
			t.Errorf("Server %s missing category", s.Name)
		}
	}
	t.Logf("Found %d recommended servers", len(servers))
}

func TestDefaultAutoConfigOptions(t *testing.T) {
	opts := DefaultAutoConfigOptions()
	if !opts.InstallMissing {
		t.Error("Expected InstallMissing to be true by default")
	}
	if len(opts.Categories) == 0 {
		t.Error("Expected default categories")
	}
}

func TestGetServersByCategory(t *testing.T) {
	categories := GetServersByCategory()
	if len(categories) == 0 {
		t.Error("Expected at least one category")
	}
	expectedCategories := []string{"search", "code", "docs", "system"}
	for _, cat := range expectedCategories {
		if servers, ok := categories[cat]; !ok || len(servers) == 0 {
			t.Errorf("Expected category '%s' to have servers", cat)
		}
	}
}

func TestGetQuickSetupServers(t *testing.T) {
	servers := GetQuickSetupServers()
	if len(servers) == 0 {
		t.Error("Expected at least one quick setup server")
	}
	t.Logf("Quick setup servers: %v", servers)
}

func TestGenerateConfigTemplate(t *testing.T) {
	template := GenerateConfigTemplate()
	if template == "" {
		t.Error("Expected non-empty config template")
	}
	if len(template) < 100 {
		t.Error("Template seems too short")
	}
}

func TestAutoConfigurator_DetectEnvironment(t *testing.T) {
	manager := NewManager()
	configurator := NewAutoConfigurator(manager, DefaultAutoConfigOptions())
	env := configurator.DetectEnvironment()
	if env.OS == "" {
		t.Error("Expected OS to be detected")
	}
	if env.Arch == "" {
		t.Error("Expected Arch to be detected")
	}
	if env.HomeDir == "" {
		t.Error("Expected HomeDir to be detected")
	}
	t.Logf("Environment: OS=%s, Arch=%s, Node=%v, NPX=%v", env.OS, env.Arch, env.HasNode, env.HasNPX)
}

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.config == nil {
		t.Error("Expected config to be initialized")
	}
	if manager.clients == nil {
		t.Error("Expected clients map to be initialized")
	}
}

func TestManagerAddRemoveServer(t *testing.T) {
	manager := NewManager()
	serverConfig := ServerConfig{
		Name:    "test-server",
		Command: "echo",
		Args:    []string{"hello"},
		Enabled: true,
	}
	manager.AddServer("test-server", serverConfig)
	if _, ok := manager.config.Servers["test-server"]; !ok {
		t.Error("Expected server to be added")
	}
	manager.RemoveServer("test-server")
	if _, ok := manager.config.Servers["test-server"]; ok {
		t.Error("Expected server to be removed")
	}
}
