#!/bin/bash

# DevOrch CLI 전수 테스트 스크립트
# QA 문서 기반 119개 명령어 체계적 검증

echo "🔍 DevOrch CLI 전수 테스트 시작..."
echo "날짜: $(date)"
echo "바이너리: ./bin/devorch --cli"
echo

# 테스트 결과 저장
RESULTS_FILE="test_results_$(date +%Y%m%d_%H%M%S).txt"
PASSED=0
FAILED=0
TOTAL=0

# 테스트 함수
test_command() {
    local cmd="$1"
    local expected_exit_code="$2"  # 0=success, 1=error_but_valid, 2=failure
    local description="$3"
    
    echo "Testing: $cmd"
    echo "Description: $description"
    
    # 명령어 실행
    echo -e "$cmd\n/exit" | ./bin/devorch --cli &>/tmp/test_output.txt
    local exit_code=$?
    local output=$(cat /tmp/test_output.txt)
    
    TOTAL=$((TOTAL + 1))
    
    # 결과 판정
    if [[ $exit_code -eq $expected_exit_code ]] || [[ $exit_code -eq 0 && $expected_exit_code -eq 1 ]]; then
        echo "✅ PASS"
        PASSED=$((PASSED + 1))
        echo "✅ $cmd | $description" >> "$RESULTS_FILE"
    else
        echo "❌ FAIL (Exit code: $exit_code)"
        FAILED=$((FAILED + 1))
        echo "❌ $cmd | $description | Exit: $exit_code" >> "$RESULTS_FILE"
        echo "   Output: $output" | head -3 >> "$RESULTS_FILE"
    fi
    echo "---"
}

echo "=== 기본 명령어 테스트 ===" >> "$RESULTS_FILE"

# Quick Commands (6개)
echo "🔸 Quick Commands (6개)"
test_command "/help" 0 "Show help"
test_command "/exit" 0 "Exit command"  
test_command "/clear" 0 "Clear screen"
test_command "/new" 1 "Create new session"
test_command "/init" 1 "Initialize system"
test_command "/history" 1 "Show command history"

# Session Management (13개)
echo "🔸 Session Management (13개)"
test_command "/session list" 0 "List all sessions"
test_command "/session create test" 0 "Create new session"
test_command "/session switch 1" 1 "Switch to session"
test_command "/session save" 1 "Save current session"
test_command "/session export" 1 "Export session data"
test_command "/session details" 1 "Show session details"
test_command "/session clear" 1 "Clear session"
test_command "/session compact" 1 "Compact session"
test_command "/session fork" 1 "Fork session"
test_command "/session merge" 1 "Merge sessions"
test_command "/session rename" 1 "Rename session"
test_command "/session delete" 1 "Delete session"
test_command "/session archive" 1 "Archive session"

# Model Management (6개)
echo "🔸 Model Management (6개)"
test_command "/model list" 1 "List available models"
test_command "/model set gpt-4" 1 "Set active model"
test_command "/model bench" 1 "Benchmark model"
test_command "/model info" 1 "Show model info"
test_command "/model switch" 1 "Switch model"
test_command "/model compare" 1 "Compare models"

# Provider Management (5개)
echo "🔸 Provider Management (5개)" 
test_command "/provider list" 1 "List providers"
test_command "/provider connect" 1 "Connect provider"
test_command "/provider disconnect" 1 "Disconnect provider"
test_command "/provider status" 1 "Provider status"
test_command "/provider test" 1 "Test provider"

# Agent Management (4개)
echo "🔸 Agent Management (4개)"
test_command "/agent list" 0 "List registered agents"
test_command "/agent set developer" 1 "Set active agent"
test_command "/agent task list" 0 "List active tasks"
test_command "/agent orchestrate test" 1 "Orchestrate task"

# Code Operations (8개)
echo "🔸 Code Operations (8개)"
test_command "/code files" 1 "List code files"
test_command "/code ls" 1 "List directory"
test_command "/code grep test" 1 "Search in code"
test_command "/code diff" 1 "Show code diff"
test_command "/code review" 1 "Code review"
test_command "/code analyze" 1 "Analyze code"
test_command "/code format" 1 "Format code"
test_command "/code test" 1 "Run tests"

# Context Management (5개)
echo "🔸 Context Management (5개)"
test_command "/context list" 1 "List context"
test_command "/context add file.txt" 1 "Add to context"
test_command "/context remove file.txt" 1 "Remove from context"
test_command "/context clear" 1 "Clear context"
test_command "/context show" 1 "Show context"

# Tools (5개)
echo "🔸 Tools (5개)"
test_command "/tool list" 0 "List available tools"
test_command "/tool info file" 1 "Show tool information"
test_command "/tool run test" 1 "Execute tool"
test_command "/tool install new" 1 "Install tool"
test_command "/tool update all" 1 "Update tools"

# Configuration (6개)
echo "🔸 Configuration (6개)"
test_command "/config list" 0 "Show configuration"
test_command "/config get theme" 1 "Get config value"
test_command "/config set theme dark" 1 "Set config value"
test_command "/config reset" 1 "Reset configuration"
test_command "/config import" 1 "Import config"
test_command "/config export" 1 "Export config"

# Authentication (4개)
echo "🔸 Authentication (4개)"
test_command "/auth login github" 1 "Login to provider"
test_command "/auth status" 1 "Show auth status"
test_command "/auth logout github" 1 "Logout from provider"
test_command "/auth refresh" 1 "Refresh tokens"

# System (6개)
echo "🔸 System (6개)"
test_command "/system info" 1 "Show system info"
test_command "/system health" 1 "System health check"
test_command "/system stats" 1 "Show statistics"
test_command "/system update" 1 "Update system"
test_command "/system backup" 1 "Backup system"
test_command "/system restore" 1 "Restore system"

# Workflow Management (6개)
echo "🔸 Workflow Management (6개)"
test_command "/workflow run test" 0 "Execute workflow"
test_command "/workflow list" 0 "List workflows"
test_command "/workflow save test" 1 "Save workflow"
test_command "/workflow delete test" 1 "Delete workflow"
test_command "/workflow export test" 1 "Export workflow"
test_command "/workflow import test" 1 "Import workflow"

# Permission Management (6개)
echo "🔸 Permission Management (6개)"
test_command "/permission list" 0 "Show permissions"
test_command "/permission grant tool admin" 0 "Grant permission"
test_command "/permission audit" 0 "Permission audit"
test_command "/permission revoke tool admin" 1 "Revoke permission"
test_command "/permission check tool" 1 "Check permission"
test_command "/permission reset" 1 "Reset permissions"

# Learning System (3개)
echo "🔸 Learning System (3개)"
test_command "/learn status" 0 "Show learning status"
test_command "/learn models" 0 "Show model performance"  
test_command "/learn feedback 5" 0 "Provide feedback"

# Custom Commands (4개)
echo "🔸 Custom Commands (4개)"
test_command "/cmd list" 0 "List custom commands"
test_command "/cmd create test script" 0 "Create custom command"
test_command "/cmd run test" 1 "Run custom command"
test_command "/cmd delete test" 1 "Delete custom command"

# Statistics (7개)
echo "🔸 Statistics (7개)"
test_command "/stats work" 0 "Work statistics"
test_command "/stats models" 0 "Model performance"
test_command "/stats quality" 0 "Quality metrics"
test_command "/stats usage" 1 "Usage statistics"
test_command "/stats performance" 1 "Performance stats"
test_command "/stats errors" 1 "Error statistics"
test_command "/stats export" 1 "Export statistics"

echo "=== Phase 4 명령어 테스트 ===" >> "$RESULTS_FILE"

# WebSocket & Real-time (5개)
echo "🔸 WebSocket & Real-time (5개)"
test_command "/ws connect ws://localhost:8080" 0 "Connect to WebSocket"
test_command "/ws list" 0 "List active connections"
test_command "/ws subscribe test" 0 "Subscribe to events"
test_command "/ws broadcast hello" 0 "Broadcast message"
test_command "/ws clients" 0 "Show connected clients"

# Execution & Runner (6개)  
echo "🔸 Execution & Runner (6개)"
test_command "/exec run echo hello" 0 "Execute command"
test_command "/exec bench test" 0 "Benchmark execution"
test_command "/exec quality 1" 0 "Check execution quality"
test_command "/exec history" 0 "Show execution history"
test_command "/exec retry 1" 0 "Retry execution"
test_command "/exec cost 1" 0 "Analyze execution cost"

# Compaction & Optimization (5개)
echo "🔸 Compaction & Optimization (5개)"
test_command "/compact analyze" 0 "Analyze compaction"
test_command "/compact session 1" 0 "Compact session"
test_command "/compact config --ratio 0.8" 0 "Set compression ratio"
test_command "/compact summary 1" 0 "View compression summary"
test_command "/compact restore 1" 0 "Restore compressed content"

# Provider Proxy & Auth (5개)
echo "🔸 Provider Proxy & Auth (5개)"
test_command "/proxy register openai" 0 "Register provider proxy"
test_command "/proxy route test" 0 "Route request"
test_command "/proxy auth inject" 0 "Inject authentication"
test_command "/proxy health" 0 "Check proxy health"
test_command "/proxy logs" 0 "View proxy logs"

# 최종 결과
echo
echo "=== 테스트 결과 요약 ==="
echo "총 테스트: $TOTAL"
echo "통과: $PASSED"
echo "실패: $FAILED"
echo "통과율: $(( PASSED * 100 / TOTAL ))%"
echo
echo "상세 결과: $RESULTS_FILE"

echo "=== 테스트 요약 ===" >> "$RESULTS_FILE"
echo "총 테스트: $TOTAL" >> "$RESULTS_FILE"
echo "통과: $PASSED" >> "$RESULTS_FILE"  
echo "실패: $FAILED" >> "$RESULTS_FILE"
echo "통과율: $(( PASSED * 100 / TOTAL ))%" >> "$RESULTS_FILE"
echo "테스트 완료 시간: $(date)" >> "$RESULTS_FILE"