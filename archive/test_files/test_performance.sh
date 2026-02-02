#!/bin/bash

# DevOrch 성능 테스트 스크립트 
# QA 문서 기준 성능 지표 측정

echo "🚀 DevOrch 성능 테스트 시작..."
echo "날짜: $(date)"
echo

PERFORMANCE_LOG="performance_results_$(date +%Y%m%d_%H%M%S).txt"

# 성능 측정 함수
measure_performance() {
    local cmd="$1"
    local description="$2"
    local target_ms="$3"
    
    echo "Testing: $cmd"
    echo "Description: $description"
    echo "Target: < ${target_ms}ms"
    
    # 5회 측정 후 평균 계산
    total_time=0
    for i in {1..5}; do
        start_time=$(date +%s%3N)
        echo -e "$cmd\n/exit" | ./bin/devorch --cli >/dev/null 2>&1
        end_time=$(date +%s%3N)
        elapsed=$((end_time - start_time))
        total_time=$((total_time + elapsed))
    done
    
    avg_time=$((total_time / 5))
    
    if [ $avg_time -lt $target_ms ]; then
        status="✅ PASS"
    else
        status="❌ FAIL"
    fi
    
    echo "Average: ${avg_time}ms $status"
    echo "$cmd | $description | ${avg_time}ms | Target: ${target_ms}ms | $status" >> "$PERFORMANCE_LOG"
    echo "---"
}

echo "=== CLI 응답 시간 테스트 ===" >> "$PERFORMANCE_LOG"

# QA 문서 기준 성능 테스트
echo "🔸 기본 명령어 성능 (목표: < 300ms)"
measure_performance "/help" "Show help" 300
measure_performance "/session list" "List sessions" 300  
measure_performance "/agent list" "List agents" 300
measure_performance "/tool list" "List tools" 300
measure_performance "/config list" "Show config" 300

echo "🔸 세션 관리 성능 (목표: < 500ms)"
measure_performance "/session create perf_test" "Create session" 500
measure_performance "/session switch 1" "Switch session" 500
measure_performance "/session save" "Save session" 500

echo "🔸 프로바이더 연결 성능 (목표: < 1000ms)"
measure_performance "/provider list" "List providers" 1000
measure_performance "/provider status" "Provider status" 1000

echo "🔸 Phase 4 WebSocket 성능 (목표: < 100ms)"
measure_performance "/ws connect test://localhost" "WebSocket connect" 100
measure_performance "/ws list" "List WS connections" 100
measure_performance "/ws clients" "List WS clients" 100

echo "🔸 Phase 4 Execution 성능 (목표: < 500ms)"
measure_performance "/exec run echo test" "Execute command" 500
measure_performance "/exec history" "Show exec history" 500
measure_performance "/exec bench test" "Benchmark execution" 500

echo "🔸 Phase 4 Compaction 성능 (목표: < 2000ms)"
measure_performance "/compact analyze" "Analyze compaction" 2000
measure_performance "/compact config" "Show compact config" 2000

echo "🔸 Phase 4 Proxy 성능 (목표: < 200ms)"
measure_performance "/proxy health" "Check proxy health" 200
measure_performance "/proxy logs" "Show proxy logs" 200

echo
echo "=== 메모리 사용량 테스트 ==="

# 메모리 사용량 측정
echo "📊 메모리 프로파일링..."
./bin/devorch --version > /dev/null 2>&1 &
DEVORCH_PID=$!
sleep 2

# macOS에서는 ps 사용
if [[ "$OSTYPE" == "darwin"* ]]; then
    MEMORY_USAGE=$(ps -o pid,rss -p $DEVORCH_PID | tail -1 | awk '{print $2}')
    MEMORY_MB=$((MEMORY_USAGE / 1024))
else
    # Linux에서는 /proc/pid/status 사용
    MEMORY_USAGE=$(cat /proc/$DEVORCH_PID/status | grep VmRSS | awk '{print $2}')
    MEMORY_MB=$((MEMORY_USAGE / 1024))
fi

kill $DEVORCH_PID 2>/dev/null

echo "💾 DevOrch 메모리 사용량: ${MEMORY_MB}MB"
echo "Memory Usage: ${MEMORY_MB}MB" >> "$PERFORMANCE_LOG"

# 메모리 기준 평가
if [ $MEMORY_MB -lt 100 ]; then
    echo "✅ 메모리 사용량 우수 (< 100MB)"
elif [ $MEMORY_MB -lt 200 ]; then
    echo "⚡ 메모리 사용량 양호 (< 200MB)"
else
    echo "⚠️  메모리 사용량 높음 (> 200MB)"
fi

echo
echo "=== 동시성 테스트 ==="

# 동시 세션 처리 테스트
echo "🔄 다중 세션 동시 처리 테스트..."
start_time=$(date +%s%3N)

for i in {1..5}; do
    echo -e "/session create concurrent_$i\n/exit" | ./bin/devorch --cli >/dev/null 2>&1 &
done

wait # 모든 백그라운드 프로세스 대기

end_time=$(date +%s%3N)
concurrent_time=$((end_time - start_time))

echo "⚡ 5개 동시 세션 생성 시간: ${concurrent_time}ms"
echo "Concurrent Sessions: ${concurrent_time}ms" >> "$PERFORMANCE_LOG"

if [ $concurrent_time -lt 2000 ]; then
    echo "✅ 동시성 성능 우수"
else
    echo "⚠️  동시성 성능 개선 필요"
fi

echo
echo "=== 성능 테스트 요약 ==="
echo "상세 결과: $PERFORMANCE_LOG"

# 결과 분석
TOTAL_TESTS=$(grep -c "Target:" "$PERFORMANCE_LOG")
PASSED_TESTS=$(grep -c "✅ PASS" "$PERFORMANCE_LOG")
PASS_RATE=$((PASSED_TESTS * 100 / TOTAL_TESTS))

echo "총 성능 테스트: $TOTAL_TESTS"
echo "통과: $PASSED_TESTS"
echo "성능 통과율: ${PASS_RATE}%"

echo "=== 성능 요약 ===" >> "$PERFORMANCE_LOG"
echo "총 테스트: $TOTAL_TESTS" >> "$PERFORMANCE_LOG"
echo "통과: $PASSED_TESTS" >> "$PERFORMANCE_LOG"
echo "통과율: ${PASS_RATE}%" >> "$PERFORMANCE_LOG"
echo "테스트 완료: $(date)" >> "$PERFORMANCE_LOG"