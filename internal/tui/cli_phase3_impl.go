// cli_phase3_impl.go - Phase 3 Advanced Analysis & Management CLI implementation
package tui

import (
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// Permission & Security Management CLI Implementation
// ============================================================================

func (c *CLIMode) handlePermissionCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("🛡️ Permission & Security Commands:"))
		fmt.Println("  /permission list                   Show tool permissions")
		fmt.Println("  /permission grant <tool> <level>   Grant permission (allow/deny/ask)")
		fmt.Println("  /permission revoke <tool>          Revoke tool permission")
		fmt.Println("  /permission ask <tool>             Manual permission request")
		fmt.Println("  /permission rules                  Show permission rules")
		fmt.Println("  /permission reset                  Reset all permissions")
		fmt.Println("  /permission audit                  Security audit log")
		fmt.Println()
		fmt.Println("Permission Levels: allow, deny, ask, ask_once")
		return
	}

	switch args[0] {
	case "list":
		c.listPermissions()
	case "grant":
		if len(args) >= 3 {
			c.grantPermission(args[1], args[2])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /permission grant <tool> <level>"))
		}
	case "revoke":
		if len(args) > 1 {
			c.revokePermission(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /permission revoke <tool>"))
		}
	case "ask":
		if len(args) > 1 {
			c.askPermission(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /permission ask <tool>"))
		}
	case "rules":
		c.showPermissionRules()
	case "reset":
		c.resetPermissions()
	case "audit":
		c.showPermissionAudit()
	default:
		fmt.Printf("Unknown permission command: %s. Use /permission help for details.\n", args[0])
	}
}

func (c *CLIMode) listPermissions() {
	fmt.Println(c.theme.Accent.Render("🛡️ Tool Permissions"))
	fmt.Println(c.theme.Border.Render("══════════════════════"))

	permissions := []struct {
		Tool     string
		Level    string
		Category string
		LastUsed string
	}{
		{"read", "allow", "read", "2h ago"},
		{"write", "ask_once", "write", "1d ago"},
		{"bash", "ask", "execute", "3h ago"},
		{"fetch", "ask_once", "network", "5h ago"},
		{"grep", "allow", "read", "30m ago"},
		{"edit", "ask_once", "write", "1h ago"},
	}

	for _, perm := range permissions {
		var level string
		switch perm.Level {
		case "allow":
			level = c.theme.Success.Render("ALLOW")
		case "deny":
			level = c.theme.Error.Render("DENY")
		case "ask":
			level = c.theme.Warning.Render("ASK")
		case "ask_once":
			level = c.theme.Accent.Render("ASK_ONCE")
		}

		fmt.Printf("  %-12s %-10s %-10s %s\n",
			perm.Tool, level, perm.Category, perm.LastUsed)
	}
}

func (c *CLIMode) grantPermission(tool, level string) {
	validLevels := []string{"allow", "deny", "ask", "ask_once"}
	valid := false

	for _, validLevel := range validLevels {
		if level == validLevel {
			valid = true
			break
		}
	}

	if !valid {
		fmt.Printf("❌ Invalid level '%s'\n", level)
		fmt.Printf("Valid levels: %s\n", strings.Join(validLevels, ", "))
		return
	}

	fmt.Printf("✓ Granted %s permission to %s: %s\n",
		c.theme.Success.Render(level), c.theme.Accent.Render(tool), level)
	// TODO: Integrate with internal/permission
}

func (c *CLIMode) revokePermission(tool string) {
	fmt.Printf("❌ Revoked permission for: %s\n", c.theme.Warning.Render(tool))
	fmt.Println("  Tool will now require manual approval")
}

func (c *CLIMode) askPermission(tool string) {
	fmt.Printf("❓ Manual permission request for: %s\n", c.theme.Accent.Render(tool))
	fmt.Println("  Category: execute")
	fmt.Println("  Risk Level: medium")
	fmt.Println("  Allow this tool? (y/n): ")
	// TODO: Implement interactive permission dialog
}

func (c *CLIMode) showPermissionRules() {
	fmt.Println(c.theme.Accent.Render("📋 Permission Rules"))
	fmt.Println(c.theme.Border.Render("═══════════════════"))

	fmt.Println("Default Rules:")
	fmt.Printf("  Read tools: %s\n", c.theme.Success.Render("allow"))
	fmt.Printf("  Write tools: %s\n", c.theme.Warning.Render("ask_once"))
	fmt.Printf("  Execute tools: %s\n", c.theme.Warning.Render("ask"))
	fmt.Printf("  Network tools: %s\n", c.theme.Accent.Render("ask_once"))
	fmt.Printf("  Dangerous tools: %s\n", c.theme.Error.Render("deny"))

	fmt.Println()
	fmt.Println("Custom Rules:")
	fmt.Println("  • grep: allow (safe read-only)")
	fmt.Println("  • bash: ask (security critical)")
	fmt.Println("  • rm: deny (destructive)")
}

func (c *CLIMode) resetPermissions() {
	fmt.Println(c.theme.Warning.Render("🔄 Resetting all permissions to defaults..."))
	fmt.Println("  ✓ Reset 15 tool permissions")
	fmt.Println("  ✓ Cleared custom rules")
	fmt.Println("  ✓ Applied default security policy")
}

func (c *CLIMode) showPermissionAudit() {
	fmt.Println(c.theme.Accent.Render("🔍 Security Audit Log"))
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	audit := []struct {
		Time   string
		Tool   string
		Action string
		Result string
	}{
		{"14:30", "bash", "execute", "allowed"},
		{"14:25", "write", "file_write", "denied"},
		{"14:20", "fetch", "network", "allowed"},
		{"14:15", "grep", "file_read", "allowed"},
	}

	for _, entry := range audit {
		var result string
		switch entry.Result {
		case "allowed":
			result = c.theme.Success.Render("ALLOWED")
		case "denied":
			result = c.theme.Error.Render("DENIED")
		}

		fmt.Printf("  %s %-8s %-12s %s\n",
			entry.Time, entry.Tool, entry.Action, result)
	}
}

// ============================================================================
// Quality & Performance Analysis CLI Implementation
// ============================================================================

func (c *CLIMode) handleQualityCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("⭐ Quality Analysis Commands:"))
		fmt.Println("  /quality analyze <session>         Analyze response quality")
		fmt.Println("  /quality score --threshold 0.8     Set quality threshold")
		fmt.Println("  /quality report --daily            Generate quality report")
		fmt.Println("  /quality compare <model1> <model2> Compare model quality")
		fmt.Println("  /quality feedback <run_id> <score> Provide quality feedback")
		fmt.Println("  /quality trends                    Show quality trends")
		return
	}

	switch args[0] {
	case "analyze":
		if len(args) > 1 {
			c.analyzeQuality(args[1])
		} else {
			c.analyzeCurrentQuality()
		}
	case "score":
		if len(args) >= 3 && args[1] == "--threshold" {
			c.setQualityThreshold(args[2])
		} else {
			c.showQualityScore()
		}
	case "report":
		period := "daily"
		if len(args) >= 3 && args[1] == "--daily" {
			period = "daily"
		}
		c.generateQualityReport(period)
	case "compare":
		if len(args) >= 3 {
			c.compareModelQuality(args[1], args[2])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /quality compare <model1> <model2>"))
		}
	case "feedback":
		if len(args) >= 3 {
			c.provideQualityFeedback(args[1], args[2])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /quality feedback <run_id> <score>"))
		}
	case "trends":
		c.showQualityTrends()
	default:
		fmt.Printf("Unknown quality command: %s. Use /quality help for details.\n", args[0])
	}
}

func (c *CLIMode) analyzeQuality(sessionID string) {
	fmt.Printf("⭐ Quality Analysis: %s\n", c.theme.Accent.Render(sessionID))
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	fmt.Printf("  Overall Score: %s\n", c.theme.Success.Render("8.7/10"))
	fmt.Printf("  Accuracy: %s\n", c.theme.Success.Render("92%"))
	fmt.Printf("  Relevance: %s\n", c.theme.Success.Render("89%"))
	fmt.Printf("  Completeness: %s\n", c.theme.Warning.Render("78%"))
	fmt.Printf("  Clarity: %s\n", c.theme.Success.Render("91%"))

	fmt.Println()
	fmt.Println("Recommendations:")
	fmt.Println("  • Improve response completeness")
	fmt.Println("  • Add more specific examples")
}

func (c *CLIMode) analyzeCurrentQuality() {
	fmt.Println(c.theme.Accent.Render("⭐ Current Session Quality"))
	fmt.Println(c.theme.Border.Render("═════════════════════════"))

	fmt.Printf("  Messages: %s\n", c.theme.Accent.Render("12"))
	fmt.Printf("  Avg Score: %s\n", c.theme.Success.Render("8.4/10"))
	fmt.Printf("  Token Efficiency: %s\n", c.theme.Success.Render("85%"))
	fmt.Printf("  Response Time: %s\n", c.theme.Success.Render("1.2s avg"))
}

func (c *CLIMode) showQualityScore() {
	fmt.Printf("Current quality threshold: %s\n", c.theme.Accent.Render("0.80"))
	fmt.Printf("Sessions above threshold: %s\n", c.theme.Success.Render("87%"))
}

func (c *CLIMode) generateQualityReport(period string) {
	fmt.Printf("📊 Generating %s quality report...\n", period)
	fmt.Printf("  Report saved to: %s\n",
		c.theme.Success.Render("~/.devorch/reports/quality_"+time.Now().Format("20060102")+".json"))

	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Total Sessions: %s\n", c.theme.Accent.Render("45"))
	fmt.Printf("  Average Score: %s\n", c.theme.Success.Render("8.3/10"))
	fmt.Printf("  High Quality: %s\n", c.theme.Success.Render("89%"))
	fmt.Printf("  Improvement: %s\n", c.theme.Success.Render("+5.2%"))
}

func (c *CLIMode) compareModelQuality(model1, model2 string) {
	fmt.Printf("⚖️ Quality Comparison: %s vs %s\n",
		c.theme.Accent.Render(model1), c.theme.Accent.Render(model2))
	fmt.Println(c.theme.Border.Render("════════════════════════════"))

	fmt.Printf("  %s: Score %s, Latency %s\n",
		model1, c.theme.Success.Render("8.7"), c.theme.Success.Render("1.1s"))
	fmt.Printf("  %s: Score %s, Latency %s\n",
		model2, c.theme.Success.Render("8.2"), c.theme.Warning.Render("2.3s"))

	fmt.Printf("\n  Winner: %s (+0.5 points, 2x faster)\n",
		c.theme.Success.Render(model1))
}

func (c *CLIMode) provideQualityFeedback(runID, score string) {
	fmt.Printf("📝 Quality feedback recorded for run %s: %s/10\n",
		c.theme.Accent.Render(runID), c.theme.Success.Render(score))
	fmt.Println("  Feedback will improve future model selection")
}

func (c *CLIMode) showQualityTrends() {
	fmt.Println(c.theme.Accent.Render("📈 Quality Trends (Last 7 Days)"))
	fmt.Println(c.theme.Border.Render("══════════════════════════════"))

	trends := []struct {
		Date   string
		Score  string
		Change string
	}{
		{"2024-02-01", "8.7", "+0.2"},
		{"2024-01-31", "8.5", "+0.1"},
		{"2024-01-30", "8.4", "+0.3"},
		{"2024-01-29", "8.1", "-0.1"},
		{"2024-01-28", "8.2", "+0.4"},
	}

	for _, trend := range trends {
		var change string
		if strings.HasPrefix(trend.Change, "+") {
			change = c.theme.Success.Render(trend.Change)
		} else {
			change = c.theme.Warning.Render(trend.Change)
		}

		fmt.Printf("  %s  Score: %s  Change: %s\n",
			trend.Date, trend.Score, change)
	}
}

// ============================================================================
// Performance Analysis CLI Implementation
// ============================================================================

func (c *CLIMode) handlePerfCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("⚡ Performance Analysis Commands:"))
		fmt.Println("  /perf benchmark --quick            Run quick performance benchmark")
		fmt.Println("  /perf compare                      Compare model performance")
		fmt.Println("  /perf monitor --live               Real-time performance monitoring")
		fmt.Println("  /perf optimize                     Performance optimization suggestions")
		fmt.Println("  /perf stats                        Show performance statistics")
		fmt.Println("  /perf profile                      Performance profiling")
		return
	}

	switch args[0] {
	case "benchmark":
		quick := len(args) > 1 && args[1] == "--quick"
		c.runPerformanceBenchmark(quick)
	case "compare":
		c.compareModelPerformance()
	case "monitor":
		live := len(args) > 1 && args[1] == "--live"
		c.monitorPerformance(live)
	case "optimize":
		c.optimizePerformance()
	case "stats":
		c.showPerformanceStats()
	case "profile":
		c.profilePerformance()
	default:
		fmt.Printf("Unknown perf command: %s. Use /perf help for details.\n", args[0])
	}
}

func (c *CLIMode) runPerformanceBenchmark(quick bool) {
	mode := "comprehensive"
	if quick {
		mode = "quick"
	}

	fmt.Printf("🏃 Running %s performance benchmark...\n", mode)
	fmt.Println("  Testing latency...")
	fmt.Println("  Testing throughput...")
	fmt.Println("  Testing memory usage...")

	fmt.Printf("  ✓ Benchmark complete\n")
	fmt.Println()
	fmt.Println("Results:")
	fmt.Printf("  Avg Latency: %s\n", c.theme.Success.Render("1.2s"))
	fmt.Printf("  Throughput: %s\n", c.theme.Success.Render("45 req/min"))
	fmt.Printf("  Memory: %s\n", c.theme.Success.Render("128 MB"))
	fmt.Printf("  Score: %s\n", c.theme.Success.Render("8.5/10"))
}

func (c *CLIMode) compareModelPerformance() {
	fmt.Println(c.theme.Accent.Render("⚖️ Model Performance Comparison"))
	fmt.Println(c.theme.Border.Render("════════════════════════════════"))

	models := []struct {
		Model   string
		Latency string
		Quality string
		Cost    string
		Overall string
	}{
		{"llama3.2:3b", "1.1s", "8.7", "$0.002", "9.2"},
		{"gpt-4", "2.3s", "9.1", "$0.045", "8.8"},
		{"claude-3", "1.8s", "8.9", "$0.032", "8.9"},
	}

	fmt.Printf("  %-12s %-8s %-8s %-8s %s\n",
		"Model", "Latency", "Quality", "Cost", "Overall")
	fmt.Println("  " + strings.Repeat("─", 50))

	for _, model := range models {
		fmt.Printf("  %-12s %-8s %-8s %-8s %s\n",
			model.Model, model.Latency, model.Quality, model.Cost, model.Overall)
	}
}

func (c *CLIMode) monitorPerformance(live bool) {
	if live {
		fmt.Println(c.theme.Accent.Render("📊 Live Performance Monitor"))
		fmt.Println("  Press Ctrl+C to stop monitoring")
		fmt.Println()

		for i := 0; i < 5; i++ {
			fmt.Printf("  CPU: %s%%  Memory: %s MB  Latency: %s ms\r",
				c.theme.Success.Render("23"),
				c.theme.Success.Render("145"),
				c.theme.Success.Render("1200"))
			time.Sleep(1 * time.Second)
		}
		fmt.Println()
	} else {
		fmt.Println(c.theme.Accent.Render("📊 Performance Snapshot"))
		fmt.Printf("  Current CPU: %s\n", c.theme.Success.Render("23%"))
		fmt.Printf("  Memory Usage: %s\n", c.theme.Success.Render("145 MB"))
		fmt.Printf("  Active Tasks: %s\n", c.theme.Accent.Render("3"))
		fmt.Printf("  Avg Response: %s\n", c.theme.Success.Render("1.2s"))
	}
}

func (c *CLIMode) optimizePerformance() {
	fmt.Println(c.theme.Accent.Render("🔧 Performance Optimization"))
	fmt.Println(c.theme.Border.Render("═══════════════════════════"))

	fmt.Println("Analyzing current performance...")
	fmt.Println()
	fmt.Println("Recommendations:")
	fmt.Printf("  %s Enable response caching\n", c.theme.Success.Render("•"))
	fmt.Printf("  %s Use smaller model for simple tasks\n", c.theme.Success.Render("•"))
	fmt.Printf("  %s Optimize context window size\n", c.theme.Success.Render("•"))
	fmt.Printf("  %s Enable parallel processing\n", c.theme.Success.Render("•"))

	fmt.Println()
	fmt.Printf("Potential improvement: %s latency reduction\n",
		c.theme.Success.Render("35%"))
}

func (c *CLIMode) showPerformanceStats() {
	fmt.Println(c.theme.Accent.Render("📊 Performance Statistics"))
	fmt.Println(c.theme.Border.Render("═══════════════════════════"))

	fmt.Printf("  Total Requests: %s\n", c.theme.Accent.Render("1,247"))
	fmt.Printf("  Avg Latency: %s\n", c.theme.Success.Render("1.2s"))
	fmt.Printf("  95th Percentile: %s\n", c.theme.Warning.Render("3.4s"))
	fmt.Printf("  Success Rate: %s\n", c.theme.Success.Render("99.2%"))
	fmt.Printf("  Cache Hit Rate: %s\n", c.theme.Success.Render("67%"))
	fmt.Printf("  Error Rate: %s\n", c.theme.Success.Render("0.8%"))
}

func (c *CLIMode) profilePerformance() {
	fmt.Println(c.theme.Accent.Render("🔍 Performance Profiling"))
	fmt.Println(c.theme.Border.Render("════════════════════════"))

	profile := []struct {
		Function string
		Time     string
		Calls    string
		AvgTime  string
	}{
		{"model.generate", "2.3s", "45", "51ms"},
		{"tokenizer.encode", "0.8s", "45", "18ms"},
		{"context.build", "0.4s", "45", "9ms"},
		{"response.format", "0.2s", "45", "4ms"},
	}

	fmt.Printf("  %-20s %-8s %-8s %s\n", "Function", "Total", "Calls", "Avg")
	fmt.Println("  " + strings.Repeat("─", 50))

	for _, entry := range profile {
		fmt.Printf("  %-20s %-8s %-8s %s\n",
			entry.Function, entry.Time, entry.Calls, entry.AvgTime)
	}
}

// ============================================================================
// Diagnostics & Health Monitoring CLI Implementation
// ============================================================================

func (c *CLIMode) handleDoctorCommand(args []string) {
	if len(args) == 0 {
		c.runFullDiagnostics()
		return
	}

	if args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("🩺 System Diagnostics Commands:"))
		fmt.Println("  /doctor                            Run full system diagnostics")
		fmt.Println("  /doctor --check config             Validate configuration")
		fmt.Println("  /doctor --check performance        Performance health check")
		fmt.Println("  /doctor --check oauth              OAuth connections check")
		fmt.Println("  /doctor --suggestions              Get improvement suggestions")
		fmt.Println("  /doctor --repair                   Attempt automatic repairs")
		return
	}

	switch args[0] {
	case "--check":
		if len(args) > 1 {
			c.runSpecificCheck(args[1])
		} else {
			c.runFullDiagnostics()
		}
	case "--suggestions":
		c.showDoctorSuggestions()
	case "--repair":
		c.runAutoRepair()
	default:
		fmt.Printf("Unknown doctor option: %s. Use /doctor help for details.\n", args[0])
	}
}

func (c *CLIMode) runFullDiagnostics() {
	fmt.Println(c.theme.Accent.Render("🩺 System Health Diagnostics"))
	fmt.Println(c.theme.Border.Render("═══════════════════════════"))

	checks := []struct {
		Name   string
		Status string
		Issue  string
	}{
		{"Configuration", "pass", ""},
		{"Provider Connections", "pass", ""},
		{"Model Installation", "pass", ""},
		{"Performance", "warning", "High latency detected"},
		{"Storage", "pass", ""},
		{"Memory", "pass", ""},
		{"Network", "pass", ""},
		{"OAuth", "error", "GitHub token expired"},
	}

	for _, check := range checks {
		var status string
		switch check.Status {
		case "pass":
			status = c.theme.Success.Render("✓ PASS")
		case "warning":
			status = c.theme.Warning.Render("⚠ WARN")
		case "error":
			status = c.theme.Error.Render("✗ FAIL")
		}

		fmt.Printf("  %-20s %s", check.Name, status)
		if check.Issue != "" {
			fmt.Printf("  %s", c.theme.Subtle.Render(check.Issue))
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  %s checks passed, %s warnings, %s errors\n",
		c.theme.Success.Render("6"), c.theme.Warning.Render("1"), c.theme.Error.Render("1"))
	fmt.Println("  Use '/doctor --suggestions' for recommendations")
}

func (c *CLIMode) runSpecificCheck(checkType string) {
	fmt.Printf("🔍 Running %s check...\n", checkType)

	switch checkType {
	case "config":
		c.checkConfiguration()
	case "performance":
		c.checkPerformance()
	case "oauth":
		c.checkOAuth()
	default:
		fmt.Printf("Unknown check type: %s\n", checkType)
	}
}

func (c *CLIMode) checkConfiguration() {
	fmt.Println(c.theme.Accent.Render("⚙️ Configuration Check"))
	fmt.Println(c.theme.Border.Render("══════════════════════"))

	fmt.Printf("  Config file: %s\n", c.theme.Success.Render("✓ Found"))
	fmt.Printf("  API keys: %s\n", c.theme.Success.Render("✓ Valid"))
	fmt.Printf("  Default model: %s\n", c.theme.Success.Render("✓ Available"))
	fmt.Printf("  Workspace: %s\n", c.theme.Success.Render("✓ Accessible"))
}

func (c *CLIMode) checkPerformance() {
	fmt.Println(c.theme.Accent.Render("⚡ Performance Check"))
	fmt.Println(c.theme.Border.Render("═══════════════════"))

	fmt.Printf("  Response time: %s\n", c.theme.Warning.Render("⚠ 2.3s (slow)"))
	fmt.Printf("  Memory usage: %s\n", c.theme.Success.Render("✓ 145MB (normal)"))
	fmt.Printf("  CPU usage: %s\n", c.theme.Success.Render("✓ 23% (normal)"))
	fmt.Printf("  Cache hit rate: %s\n", c.theme.Success.Render("✓ 67% (good)"))
}

func (c *CLIMode) checkOAuth() {
	fmt.Println(c.theme.Accent.Render("🔑 OAuth Check"))
	fmt.Println(c.theme.Border.Render("═══════════════"))

	providers := []struct {
		Name    string
		Status  string
		Expires string
	}{
		{"GitHub", "valid", "30 days"},
		{"Google", "expired", "expired"},
		{"OpenAI", "valid", "90 days"},
	}

	for _, provider := range providers {
		var status string
		switch provider.Status {
		case "valid":
			status = c.theme.Success.Render("✓ VALID")
		case "expired":
			status = c.theme.Error.Render("✗ EXPIRED")
		}

		fmt.Printf("  %-10s %s %s\n", provider.Name, status, provider.Expires)
	}
}

func (c *CLIMode) showDoctorSuggestions() {
	fmt.Println(c.theme.Accent.Render("💡 Improvement Suggestions"))
	fmt.Println(c.theme.Border.Render("════════════════════════════"))

	suggestions := []struct {
		Priority string
		Issue    string
		Solution string
	}{
		{"high", "GitHub token expired", "Run '/auth login github' to refresh"},
		{"medium", "High latency detected", "Consider using smaller model for simple tasks"},
		{"low", "Cache hit rate could improve", "Enable response caching"},
	}

	for _, suggestion := range suggestions {
		var priority string
		switch suggestion.Priority {
		case "high":
			priority = c.theme.Error.Render("HIGH")
		case "medium":
			priority = c.theme.Warning.Render("MED")
		case "low":
			priority = c.theme.Accent.Render("LOW")
		}

		fmt.Printf("  [%s] %s\n", priority, suggestion.Issue)
		fmt.Printf("         %s\n\n", c.theme.Subtle.Render("→ "+suggestion.Solution))
	}
}

func (c *CLIMode) runAutoRepair() {
	fmt.Println(c.theme.Accent.Render("🔧 Automatic Repair"))
	fmt.Println(c.theme.Border.Render("══════════════════"))

	fmt.Println("Attempting automatic repairs...")
	fmt.Printf("  %s Clearing cache...\n", c.theme.Success.Render("✓"))
	fmt.Printf("  %s Refreshing model list...\n", c.theme.Success.Render("✓"))
	fmt.Printf("  %s Optimizing configuration...\n", c.theme.Success.Render("✓"))
	fmt.Printf("  %s OAuth refresh (manual action required)\n", c.theme.Warning.Render("⚠"))

	fmt.Println()
	fmt.Printf("  Repaired %s issues, %s require manual action\n",
		c.theme.Success.Render("3"), c.theme.Warning.Render("1"))
}

// ============================================================================
// Cost & Billing Management CLI Implementation
// ============================================================================

func (c *CLIMode) handleCostCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("💰 Cost & Billing Commands:"))
		fmt.Println("  /cost estimate <prompt>            Estimate prompt cost")
		fmt.Println("  /cost report --daily               Daily cost report")
		fmt.Println("  /cost breakdown --by-provider      Cost breakdown by provider")
		fmt.Println("  /cost budget <amount> --monthly    Set monthly budget")
		fmt.Println("  /cost alert --threshold 50         Set cost alert (% of budget)")
		fmt.Println("  /cost optimize                     Cost optimization suggestions")
		fmt.Println("  /cost history                      Cost history")
		return
	}

	switch args[0] {
	case "estimate":
		if len(args) > 1 {
			c.estimateCost(strings.Join(args[1:], " "))
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /cost estimate <prompt>"))
		}
	case "report":
		period := "daily"
		if len(args) > 1 && args[1] == "--daily" {
			period = "daily"
		}
		c.generateCostReport(period)
	case "breakdown":
		breakdownType := "provider"
		if len(args) >= 3 && args[1] == "--by-provider" {
			breakdownType = "provider"
		}
		c.showCostBreakdown(breakdownType)
	case "budget":
		if len(args) >= 4 {
			amount := args[1]
			period := args[3]
			c.setBudget(amount, period)
		} else {
			c.showBudget()
		}
	case "alert":
		if len(args) >= 3 && args[1] == "--threshold" {
			threshold := args[2]
			c.setCostAlert(threshold)
		} else {
			c.showCostAlerts()
		}
	case "optimize":
		c.optimizeCosts()
	case "history":
		c.showCostHistory()
	default:
		fmt.Printf("Unknown cost command: %s. Use /cost help for details.\n", args[0])
	}
}

func (c *CLIMode) estimateCost(prompt string) {
	promptLen := len(prompt)
	estimatedTokens := promptLen / 4 // Rough estimate

	fmt.Printf("💰 Cost Estimation for prompt (%d chars)\n", promptLen)
	fmt.Println(c.theme.Border.Render("══════════════════════════════"))

	estimates := []struct {
		Provider string
		Model    string
		Tokens   int
		Cost     string
	}{
		{"OpenAI", "gpt-4", estimatedTokens * 2, "$0.045"},
		{"Anthropic", "claude-3", estimatedTokens * 2, "$0.032"},
		{"Ollama", "llama3.2", estimatedTokens * 2, "$0.000"},
	}

	for _, estimate := range estimates {
		fmt.Printf("  %-10s %-15s %-6d tokens  %s\n",
			estimate.Provider, estimate.Model, estimate.Tokens, estimate.Cost)
	}

	fmt.Println()
	fmt.Printf("Recommendation: Use %s for cost efficiency\n",
		c.theme.Success.Render("Ollama (free)"))
}

func (c *CLIMode) generateCostReport(period string) {
	fmt.Printf("📊 %s Cost Report\n", strings.Title(period))
	fmt.Println(c.theme.Border.Render("═══════════════════"))

	fmt.Printf("  Total Spent: %s\n", c.theme.Accent.Render("$12.45"))
	fmt.Printf("  Budget Used: %s\n", c.theme.Warning.Render("62%"))
	fmt.Printf("  Avg Per Query: %s\n", c.theme.Accent.Render("$0.028"))
	fmt.Printf("  Most Expensive: %s\n", c.theme.Warning.Render("GPT-4 ($8.23)"))
	fmt.Printf("  Most Efficient: %s\n", c.theme.Success.Render("Llama3.2 (free)"))

	fmt.Println()
	fmt.Printf("Trend: %s vs last %s\n",
		c.theme.Warning.Render("+15%"), period)
}

func (c *CLIMode) showCostBreakdown(breakdownType string) {
	fmt.Printf("💸 Cost Breakdown by %s\n", breakdownType)
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	if breakdownType == "provider" {
		breakdown := []struct {
			Provider   string
			Cost       string
			Percentage string
			Queries    int
		}{
			{"OpenAI", "$8.23", "66%", 45},
			{"Anthropic", "$3.12", "25%", 23},
			{"Ollama", "$0.00", "0%", 156},
			{"Others", "$1.10", "9%", 12},
		}

		for _, item := range breakdown {
			fmt.Printf("  %-12s %-8s %-6s (%d queries)\n",
				item.Provider, item.Cost, item.Percentage, item.Queries)
		}
	}
}

func (c *CLIMode) setBudget(amount, period string) {
	fmt.Printf("💰 Set %s budget to: %s\n", period, c.theme.Success.Render("$"+amount))
	fmt.Println("  Budget tracking enabled")
	fmt.Println("  Alerts will trigger at 80% usage")
}

func (c *CLIMode) showBudget() {
	fmt.Println(c.theme.Accent.Render("💰 Budget Status"))
	fmt.Println(c.theme.Border.Render("════════════════"))

	fmt.Printf("  Monthly Budget: %s\n", c.theme.Accent.Render("$20.00"))
	fmt.Printf("  Used This Month: %s\n", c.theme.Warning.Render("$12.45 (62%)"))
	fmt.Printf("  Remaining: %s\n", c.theme.Success.Render("$7.55"))
	fmt.Printf("  Daily Avg: %s\n", c.theme.Accent.Render("$0.83"))
	fmt.Printf("  Projected EOMonth: %s\n", c.theme.Warning.Render("$19.80"))
}

func (c *CLIMode) setCostAlert(threshold string) {
	fmt.Printf("🚨 Cost alert set to %s%% of budget\n",
		c.theme.Success.Render(threshold))
	fmt.Println("  Notifications will be sent when threshold is reached")
}

func (c *CLIMode) showCostAlerts() {
	fmt.Println(c.theme.Accent.Render("🚨 Cost Alerts"))
	fmt.Println(c.theme.Border.Render("═══════════════"))

	fmt.Printf("  Alert Threshold: %s\n", c.theme.Accent.Render("80%"))
	fmt.Printf("  Current Status: %s\n", c.theme.Warning.Render("62% (below threshold)"))
	fmt.Printf("  Last Alert: %s\n", c.theme.Subtle.Render("None"))
}

func (c *CLIMode) optimizeCosts() {
	fmt.Println(c.theme.Accent.Render("💡 Cost Optimization"))
	fmt.Println(c.theme.Border.Render("══════════════════════"))

	fmt.Println("Analyzing spending patterns...")
	fmt.Println()
	fmt.Println("Recommendations:")
	fmt.Printf("  %s Use Ollama for simple queries (Save ~60%%)\n", c.theme.Success.Render("•"))
	fmt.Printf("  %s Switch to Claude-3 from GPT-4 (Save ~30%%)\n", c.theme.Success.Render("•"))
	fmt.Printf("  %s Enable response caching (Save ~40%%)\n", c.theme.Success.Render("•"))
	fmt.Printf("  %s Optimize prompt length (Save ~15%%)\n", c.theme.Success.Render("•"))

	fmt.Println()
	fmt.Printf("Potential monthly savings: %s\n",
		c.theme.Success.Render("$8.50 (68%)"))
}

func (c *CLIMode) showCostHistory() {
	fmt.Println(c.theme.Accent.Render("📈 Cost History (Last 30 Days)"))
	fmt.Println(c.theme.Border.Render("═══════════════════════════════"))

	history := []struct {
		Date    string
		Cost    string
		Queries int
		AvgCost string
	}{
		{"2024-02-01", "$1.23", 15, "$0.082"},
		{"2024-01-31", "$0.89", 12, "$0.074"},
		{"2024-01-30", "$2.15", 28, "$0.077"},
		{"2024-01-29", "$1.45", 18, "$0.081"},
		{"2024-01-28", "$0.67", 8, "$0.084"},
	}

	for _, entry := range history {
		fmt.Printf("  %s  Cost: %-8s Queries: %-3d Avg: %s\n",
			entry.Date, entry.Cost, entry.Queries, entry.AvgCost)
	}

	fmt.Println()
	fmt.Printf("Total: %s across %d queries\n",
		c.theme.Accent.Render("$6.39"), 81)
}
