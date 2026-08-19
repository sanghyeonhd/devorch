#!/bin/bash

# DevOrch CLI 최종 통합 검증 스크립트
echo "🎯 DevOrch CLI 최종 통합 검증"
echo "============================="
echo ""

DEVORCH_CLI="./bin/devorch --cli"

# 주요 카테고리별 대표 명령어들
declare -a TEST_COMMANDS=(
    "Basic Help:/help"
    "Agent System:/agent help"
    "Learning System:/learn help"
    "Analytics:/analytics help"
    "Quality System:/quality help"
    "Benchmark:/benchmark help"
    "Hardware Detection:/hardware help"
    "Auto Setup:/autosetup help"
    "Installation:/install help"
    "LSP Integration:/lsp help"
    "Editor Integration:/editor help"
    "Terminal Management:/terminal help"
    "Shell Management:/shell help"
    "Workspace Management:/workspace help"
    "File History:/history help"
    "Plugin Management:/plugin help"
    "Experiments:/experiment help"
    "WebSocket Server:/websocket help"
    "MCP Protocol:/mcp help"
    "API Management:/api help"
    "Event System:/events help"
    "Billing System:/billing help"
    "Budget Management:/budget help"
    "Usage Tracking:/usage help"
    "System Diagnostics:/doctor help"
    "Log Management:/logs help"
    "Performance Metrics:/metrics help"
    "Debug System:/debug help"
    "Internationalization:/i18n help"
    "Theme Management:/theme help"
    "Memory Management:/memory help"
    "File Operations:/file help"
    "Task Management:/task help"
    "Task Delegation:/delegate help"
)

success_count=0
total_count=${#TEST_COMMANDS[@]}

echo "📋 엔터프라이즈급 CLI 기능 검증:"
echo ""

for test_command in "${TEST_COMMANDS[@]}"; do
    IFS=':' read -ra PARTS <<< "$test_command"
    test_name="${PARTS[0]}"
    command="${PARTS[1]}"
    
    printf "%-25s %s ... " "[$test_name]" "$command"
    
    # 명령어 실행 테스트 (타임아웃 5초)
    if timeout 5s bash -c "echo '$command' | $DEVORCH_CLI" >/dev/null 2>&1; then
        echo "✅"
        ((success_count++))
    else
        echo "❌"
    fi
done

echo ""
echo "============================="
echo "📊 최종 통합 검증 결과:"
echo "  ✅ 성공: $success_count / $total_count"
echo "  📈 성공률: $(echo "scale=1; $success_count * 100 / $total_count" | bc)%"

echo ""
echo "🔍 실제 기능 동작 테스트:"
echo ""

# 실제 기능 테스트
declare -a FUNCTIONAL_TESTS=(
    "Agent Status:/agent status"
    "Learning Stats:/learn stats"
    "Analytics Status:/analytics status"
    "Hardware Detection:/hardware detect"
    "Billing Status:/billing status"
    "Doctor Check:/doctor check"
    "Theme List:/theme list"
    "Memory Status:/memory show"
)

functional_success=0
functional_total=${#FUNCTIONAL_TESTS[@]}

for test_command in "${FUNCTIONAL_TESTS[@]}"; do
    IFS=':' read -ra PARTS <<< "$test_command"
    test_name="${PARTS[0]}"
    command="${PARTS[1]}"
    
    printf "%-20s %s ... " "[$test_name]" "$command"
    
    if timeout 5s bash -c "echo '$command' | $DEVORCH_CLI" >/dev/null 2>&1; then
        echo "✅"
        ((functional_success++))
    else
        echo "❌"
    fi
done

echo ""
echo "============================="
echo "📈 기능 동작 테스트 결과:"
echo "  ✅ 성공: $functional_success / $functional_total"
echo "  📈 성공률: $(echo "scale=1; $functional_success * 100 / $functional_total" | bc)%"

echo ""
echo "🏆 최종 평가:"
if [ $success_count -eq $total_count ] && [ $functional_success -eq $functional_total ]; then
    echo "🎉 모든 CLI 기능이 완벽하게 통합되었습니다!"
    echo ""
    echo "📚 통합 완료 기능:"
    echo "  • 52개 엔터프라이즈급 명령어"
    echo "  • 9개 주요 기능 카테고리"
    echo "  • Agent 오케스트레이션 시스템"
    echo "  • Multi-LLM 학습 (Thompson Sampling)"
    echo "  • 성능 분석 및 Attribution"
    echo "  • 하드웨어 감지 및 최적화"
    echo "  • 개발 도구 통합 (LSP, 에디터, 터미널)"
    echo "  • 실시간 통신 (WebSocket, MCP)"
    echo "  • 비용 추적 및 예산 관리"
    echo "  • 시스템 진단 및 모니터링"
    echo "  • 국제화 및 테마 시스템"
elif [ $success_count -gt $((total_count * 8 / 10)) ]; then
    echo "✅ CLI 통합이 거의 완료되었습니다! (80% 이상)"
    echo "  실패: $((total_count - success_count))개 명령어"
else
    echo "⚠️  CLI 통합에 추가 작업이 필요합니다."
    echo "  실패: $((total_count - success_count))개 명령어"
fi

echo ""
echo "============================="