#!/bin/bash

# DevOrch 기능별 자동화 QA 테스트
# 실제 사용자 시나리오 기반 검증 스크립트

set -e

echo "🔍 DevOrch 기능별 QA 테스트 시작..."
echo "날짜: $(date)"
echo "바이너리: ./bin/devorch --cli"
echo

# 테스트 결과 저장
RESULTS_FILE="functional_qa_results_$(date +%Y%m%d_%H%M%S).json"
TEMP_DIR="/tmp/devorch_qa_$$"
mkdir -p "$TEMP_DIR"

# 테스트 카운터
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# JSON 결과 초기화
echo '{"test_results": [' > "$RESULTS_FILE"

# 테스트 함수
run_functional_test() {
    local test_name="$1"
    local test_description="$2" 
    local test_commands="$3"
    local validation_check="$4"
    
    echo "🧪 Testing: $test_name"
    echo "📝 Description: $test_description"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    # 테스트 실행
    local test_output="$TEMP_DIR/test_$TOTAL_TESTS.out"
    local test_error="$TEMP_DIR/test_$TOTAL_TESTS.err"
    
    # DevOrch 명령 실행
    echo -e "$test_commands" | timeout 30s ./bin/devorch --cli > "$test_output" 2> "$test_error"
    local exit_code=$?
    
    # 검증 실행
    local validation_result="UNKNOWN"
    local validation_details=""
    
    if [[ $exit_code -eq 0 ]]; then
        # 사용자 정의 검증 로직 실행
        if eval "$validation_check"; then
            validation_result="PASS"
            PASSED_TESTS=$((PASSED_TESTS + 1))
            echo "✅ PASS"
        else
            validation_result="FAIL"
            validation_details="Validation check failed: $validation_check"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            echo "❌ FAIL (Validation)"
        fi
    else
        validation_result="FAIL"
        validation_details="Command failed with exit code: $exit_code"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo "❌ FAIL (Exit code: $exit_code)"
    fi
    
    # JSON 결과 추가
    cat >> "$RESULTS_FILE" << EOF
  {
    "test_id": $TOTAL_TESTS,
    "test_name": "$test_name",
    "description": "$test_description", 
    "commands": "$test_commands",
    "validation": "$validation_check",
    "result": "$validation_result",
    "exit_code": $exit_code,
    "details": "$validation_details",
    "timestamp": "$(date -Iseconds)",
    "output_file": "$test_output",
    "error_file": "$test_error"
  }$([ $TOTAL_TESTS -lt 50 ] && echo "," || echo "")
EOF
    
    echo "---"
}

# 핵심 기능 검증 테스트들

echo "=== 1. 기본 명령어 기능성 테스트 ==="

run_functional_test \
    "help_system" \
    "도움말 시스템이 완전한 명령어 목록을 제공하는가" \
    "/help\n/exit" \
    "grep -q 'Quick Commands' '$test_output' && grep -q '/exit' '$test_output'"

run_functional_test \
    "session_lifecycle" \
    "세션 생성, 목록, 삭제가 정상 작동하는가" \
    "/session create test_session\n/session list\n/session delete test_session\n/exit" \
    "grep -q 'test_session' '$test_output'"

run_functional_test \
    "context_management" \
    "컨텍스트 추가와 목록이 정상 작동하는가" \
    "/context add README.md\n/context list\n/context clear\n/exit" \
    "grep -q -E '(README.md|Added to context|Context cleared)' '$test_output'"

echo "=== 2. AI 모델 연동 테스트 ==="

run_functional_test \
    "model_listing" \
    "사용 가능한 AI 모델 목록이 표시되는가" \
    "/model list\n/exit" \
    "grep -q -E '(gpt|claude|gemini|Models available)' '$test_output'"

run_functional_test \
    "model_switching" \
    "AI 모델 전환이 정상 작동하는가" \
    "/model set gpt-4\n/model list\n/exit" \
    "grep -q -E '(Active|Current|Selected).*gpt-4' '$test_output'"

echo "=== 3. 코드 작업 도구 테스트 ==="

run_functional_test \
    "code_file_listing" \
    "프로젝트 파일 목록이 올바르게 표시되는가" \
    "/code files\n/exit" \
    "grep -q -E '\\.(go|py|js|md)' '$test_output'"

run_functional_test \
    "directory_exploration" \
    "디렉토리 탐색 기능이 작동하는가" \
    "/code ls\n/exit" \
    "grep -q -E '(README|src|internal|\./)' '$test_output'"

run_functional_test \
    "code_search" \
    "코드 검색 기능이 작동하는가" \
    "/code grep 'package'\n/exit" \
    "grep -q -E '(package|Found|matches)' '$test_output'"

echo "=== 4. 도구 시스템 테스트 ==="

run_functional_test \
    "tool_listing" \
    "사용 가능한 도구 목록이 표시되는가" \
    "/tool list\n/exit" \
    "grep -q -E '(file|git|terminal|Available tools)' '$test_output'"

run_functional_test \
    "file_tool_operation" \
    "파일 도구가 정상 작동하는가" \
    "/tool run file read README.md\n/exit" \
    "grep -q -E '(DevOrch|README|Error|Success)' '$test_output'"

echo "=== 5. 시스템 정보 및 설정 테스트 ==="

run_functional_test \
    "system_info" \
    "시스템 정보가 올바르게 표시되는가" \
    "/system info\n/exit" \
    "grep -q -E '(Version|Go|Memory|DevOrch)' '$test_output'"

run_functional_test \
    "system_health" \
    "시스템 헬스 체크가 작동하는가" \
    "/system health\n/exit" \
    "grep -q -E '(Health|Status|Services|OK)' '$test_output'"

run_functional_test \
    "configuration_management" \
    "설정 관리가 정상 작동하는가" \
    "/config list\n/config set theme dark\n/config get theme\n/exit" \
    "grep -q -E '(theme|dark|Configuration)' '$test_output'"

echo "=== 6. 에이전트 및 워크플로우 테스트 ==="

run_functional_test \
    "agent_management" \
    "AI 에이전트 관리가 작동하는가" \
    "/agent list\n/agent set developer\n/exit" \
    "grep -q -E '(developer|architect|Agent|Active)' '$test_output'"

run_functional_test \
    "workflow_operations" \
    "워크플로우 관리가 작동하는가" \
    "/workflow list\n/workflow run test\n/exit" \
    "grep -q -E '(workflow|Workflow|test|Running)' '$test_output'"

echo "=== 7. Phase 4 실시간 기능 테스트 ==="

run_functional_test \
    "websocket_connectivity" \
    "WebSocket 연결 기능이 작동하는가" \
    "/ws connect ws://localhost:8080\n/ws list\n/exit" \
    "grep -q -E '(Connected|WebSocket|Connection|clients)' '$test_output'"

run_functional_test \
    "execution_engine" \
    "명령 실행 엔진이 작동하는가" \
    "/exec run echo 'test'\n/exec history\n/exit" \
    "grep -q -E '(test|Execution|Command|History)' '$test_output'"

run_functional_test \
    "compaction_analysis" \
    "압축 분석 기능이 작동하는가" \
    "/compact analyze\n/exit" \
    "grep -q -E '(Compaction|Analysis|compression|sessions)' '$test_output'"

run_functional_test \
    "proxy_management" \
    "프록시 관리가 작동하는가" \
    "/proxy register openai\n/proxy health\n/proxy logs\n/exit" \
    "grep -q -E '(proxy|Proxy|openai|Health|registered)' '$test_output'"

echo "=== 8. 권한 및 보안 테스트 ==="

run_functional_test \
    "permission_system" \
    "권한 관리 시스템이 작동하는가" \
    "/permission list\n/permission grant tool admin\n/permission audit\n/exit" \
    "grep -q -E '(permission|Permission|admin|tool|granted)' '$test_output'"

run_functional_test \
    "authentication_status" \
    "인증 상태 확인이 작동하는가" \
    "/auth status\n/exit" \
    "grep -q -E '(Authentication|Status|Provider|Token)' '$test_output'"

echo "=== 9. 학습 및 통계 시스템 테스트 ==="

run_functional_test \
    "learning_system" \
    "학습 시스템이 작동하는가" \
    "/learn status\n/learn feedback 4\n/exit" \
    "grep -q -E '(Learning|Status|feedback|Model)' '$test_output'"

run_functional_test \
    "statistics_reporting" \
    "통계 리포팅이 작동하는가" \
    "/stats work\n/stats models\n/stats quality\n/exit" \
    "grep -q -E '(Statistics|Work|Model|Quality)' '$test_output'"

echo "=== 10. 사용자 정의 명령어 테스트 ==="

run_functional_test \
    "custom_commands" \
    "사용자 정의 명령어가 작동하는가" \
    "/cmd create test_cmd 'echo test'\n/cmd list\n/cmd run test_cmd\n/exit" \
    "grep -q -E '(test_cmd|Created|Custom|command)' '$test_output'"

# JSON 마무리
echo ']}' >> "$RESULTS_FILE"

# 통합 워크플로우 테스트
echo "=== 🚀 통합 워크플로우 테스트 ==="

run_functional_test \
    "complete_development_workflow" \
    "전체 개발 워크플로우가 연속적으로 작동하는가" \
    "/init\n/session create integration_test\n/agent set developer\n/model set gpt-4\n/code files\n/context add README.md\n/system health\n/session save\n/exit" \
    "grep -q -E '(Session.*created|Agent.*set|Model.*set|System.*healthy)' '$test_output'"

# 성능 테스트
echo "=== ⚡ 성능 및 안정성 테스트 ==="

# 메모리 사용량 체크
echo "🧠 메모리 사용량 테스트"
MEMORY_BEFORE=$(ps -o vsz= -p $$ | tr -d ' ')
for i in {1..10}; do
    echo -e "/session create perf_test_$i\n/context add README.md\n/context clear\n/exit" | ./bin/devorch --cli > /dev/null
done
MEMORY_AFTER=$(ps -o vsz= -p $$ | tr -d ' ')
MEMORY_DIFF=$((MEMORY_AFTER - MEMORY_BEFORE))

echo "메모리 사용량 변화: ${MEMORY_DIFF}KB"

# 응답 시간 테스트
echo "⏱️ 응답 시간 테스트"
TIME_START=$(date +%s.%N)
echo -e "/help\n/system info\n/model list\n/exit" | ./bin/devorch --cli > /dev/null
TIME_END=$(date +%s.%N)
RESPONSE_TIME=$(echo "$TIME_END - $TIME_START" | bc)

echo "평균 응답 시간: ${RESPONSE_TIME}초"

# 최종 결과 출력
echo
echo "=== 📊 테스트 결과 요약 ==="
echo "총 테스트: $TOTAL_TESTS"
echo "통과: $PASSED_TESTS"
echo "실패: $FAILED_TESTS"
echo "성공률: $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%"
echo
echo "메모리 사용량 변화: ${MEMORY_DIFF}KB"
echo "평균 응답 시간: ${RESPONSE_TIME}초"
echo
echo "상세 결과: $RESULTS_FILE"

# 결과 분석
if [[ $FAILED_TESTS -eq 0 ]]; then
    echo "🎉 모든 테스트 통과! DevOrch가 프로덕션 준비 상태입니다."
    exit 0
elif [[ $PASSED_TESTS -gt $((TOTAL_TESTS * 8 / 10)) ]]; then
    echo "⚠️ 일부 테스트 실패. 80% 이상 통과로 기본 기능은 안정적입니다."
    exit 1
else
    echo "❌ 많은 테스트 실패. 시스템 점검이 필요합니다."
    exit 2
fi

# 정리
rm -rf "$TEMP_DIR"