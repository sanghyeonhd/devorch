#!/bin/bash

# DevOrch QA 결과 분석 도구
# JSON 결과 파일을 분석하여 상세 리포트 생성

if [[ $# -eq 0 ]]; then
    echo "사용법: $0 <qa_results.json>"
    echo "예시: $0 functional_qa_results_20260202_143000.json"
    exit 1
fi

QA_RESULTS_FILE="$1"

if [[ ! -f "$QA_RESULTS_FILE" ]]; then
    echo "❌ 결과 파일을 찾을 수 없습니다: $QA_RESULTS_FILE"
    exit 1
fi

echo "📊 DevOrch QA 결과 분석 리포트"
echo "날짜: $(date)"
echo "결과 파일: $QA_RESULTS_FILE"
echo "=================================="

# jq가 설치되어 있는지 확인
if ! command -v jq &> /dev/null; then
    echo "⚠️ jq가 설치되어 있지 않습니다. 기본 분석만 수행합니다."
    
    # 기본 통계
    total_tests=$(grep -o '"test_id"' "$QA_RESULTS_FILE" | wc -l)
    passed_tests=$(grep -o '"result": "PASS"' "$QA_RESULTS_FILE" | wc -l)
    failed_tests=$(grep -o '"result": "FAIL"' "$QA_RESULTS_FILE" | wc -l)
    
    echo "총 테스트: $total_tests"
    echo "통과: $passed_tests"
    echo "실패: $failed_tests"
    echo "성공률: $(( passed_tests * 100 / total_tests ))%"
    
    echo
    echo "실패한 테스트들:"
    grep -B2 -A2 '"result": "FAIL"' "$QA_RESULTS_FILE" | grep '"test_name"' | sed 's/.*"test_name": "\([^"]*\)".*/- \1/'
    
    exit 0
fi

# jq를 사용한 상세 분석
echo "📈 전체 통계"
echo "============"

total_tests=$(jq '.test_results | length' "$QA_RESULTS_FILE")
passed_tests=$(jq '[.test_results[] | select(.result == "PASS")] | length' "$QA_RESULTS_FILE")
failed_tests=$(jq '[.test_results[] | select(.result == "FAIL")] | length' "$QA_RESULTS_FILE")
success_rate=$(( passed_tests * 100 / total_tests ))

echo "총 테스트 수행: $total_tests"
echo "통과: $passed_tests ✅"
echo "실패: $failed_tests ❌"
echo "성공률: $success_rate%"

# 성공률에 따른 평가
if [[ $success_rate -ge 95 ]]; then
    echo "🎉 우수 (95% 이상) - 프로덕션 배포 가능"
elif [[ $success_rate -ge 90 ]]; then
    echo "✅ 양호 (90-94%) - 일부 개선 후 배포 가능"
elif [[ $success_rate -ge 80 ]]; then
    echo "⚠️ 보통 (80-89%) - 개선 필요"
else
    echo "❌ 불량 (80% 미만) - 대폭 개선 필요"
fi

echo
echo "📋 카테고리별 분석"
echo "=================="

# 카테고리별 성공률 분석 (테스트 이름 패턴 기반)
declare -A categories=(
    ["basic"]="기본 기능"
    ["session"]="세션 관리" 
    ["model"]="AI 모델"
    ["code"]="코드 작업"
    ["tool"]="도구 시스템"
    ["system"]="시스템"
    ["agent"]="에이전트"
    ["workflow"]="워크플로우"
    ["websocket"]="WebSocket"
    ["execution"]="실행 엔진"
    ["compaction"]="압축"
    ["proxy"]="프록시"
    ["permission"]="권한"
    ["learning"]="학습"
    ["custom"]="사용자 정의"
)

for category in "${!categories[@]}"; do
    category_total=$(jq "[.test_results[] | select(.test_name | contains(\"$category\"))] | length" "$QA_RESULTS_FILE")
    category_passed=$(jq "[.test_results[] | select(.test_name | contains(\"$category\") and .result == \"PASS\")] | length" "$QA_RESULTS_FILE")
    
    if [[ $category_total -gt 0 ]]; then
        category_rate=$(( category_passed * 100 / category_total ))
        printf "%-15s: %2d/%2d (%3d%%) %s\n" "${categories[$category]}" "$category_passed" "$category_total" "$category_rate" \
            $(if [[ $category_rate -ge 90 ]]; then echo "✅"; elif [[ $category_rate -ge 70 ]]; then echo "⚠️"; else echo "❌"; fi)
    fi
done

echo
echo "❌ 실패한 테스트 상세"
echo "===================="

jq -r '.test_results[] | select(.result == "FAIL") | 
    "테스트: \(.test_name)\n설명: \(.description)\n실패 원인: \(.details)\n종료 코드: \(.exit_code)\n시간: \(.timestamp)\n" + "-" * 50' "$QA_RESULTS_FILE"

echo
echo "🚨 주요 이슈 분석"
echo "================="

# 종료 코드별 실패 분류
echo "종료 코드별 실패 분류:"
jq -r '.test_results[] | select(.result == "FAIL") | .exit_code' "$QA_RESULTS_FILE" | sort | uniq -c | while read count code; do
    echo "  Exit Code $code: $count cases"
done

echo
echo "공통 실패 패턴:"
jq -r '.test_results[] | select(.result == "FAIL") | .details' "$QA_RESULTS_FILE" | grep -o '[A-Za-z][a-z]*' | sort | uniq -c | sort -nr | head -5 | while read count pattern; do
    echo "  '$pattern': $count occurrences"
done

echo
echo "📊 성능 분석"
echo "============="

# 평균 실행 시간 분석 (타임스탬프 기반 추정)
first_test_time=$(jq -r '.test_results[0].timestamp' "$QA_RESULTS_FILE")
last_test_time=$(jq -r '.test_results[-1].timestamp' "$QA_RESULTS_FILE")

if command -v date &> /dev/null; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        first_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S%z" "$first_test_time" "+%s" 2>/dev/null || echo "0")
        last_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S%z" "$last_test_time" "+%s" 2>/dev/null || echo "0")
    else
        # Linux
        first_epoch=$(date -d "$first_test_time" "+%s" 2>/dev/null || echo "0")
        last_epoch=$(date -d "$last_test_time" "+%s" 2>/dev/null || echo "0")
    fi
    
    if [[ $first_epoch != "0" && $last_epoch != "0" ]]; then
        total_duration=$((last_epoch - first_epoch))
        avg_test_time=$((total_duration / total_tests))
        echo "총 실행 시간: ${total_duration}초"
        echo "평균 테스트 시간: ${avg_test_time}초"
    fi
fi

echo
echo "💡 권장사항"
echo "============"

if [[ $failed_tests -gt 0 ]]; then
    echo "실패한 테스트 개선 방안:"
    
    # 세션 관련 실패가 많은 경우
    session_failures=$(jq '[.test_results[] | select(.test_name | contains("session") and .result == "FAIL")] | length' "$QA_RESULTS_FILE")
    if [[ $session_failures -gt 2 ]]; then
        echo "  🔄 세션 관리: 세션 상태 관리 로직 점검 필요"
    fi
    
    # 모델 관련 실패가 있는 경우  
    model_failures=$(jq '[.test_results[] | select(.test_name | contains("model") and .result == "FAIL")] | length' "$QA_RESULTS_FILE")
    if [[ $model_failures -gt 0 ]]; then
        echo "  🤖 AI 모델: API 연결 및 인증 상태 확인 필요"
    fi
    
    # 시스템 관련 실패가 있는 경우
    system_failures=$(jq '[.test_results[] | select(.test_name | contains("system") and .result == "FAIL")] | length' "$QA_RESULTS_FILE")
    if [[ $system_failures -gt 0 ]]; then
        echo "  ⚙️ 시스템: 기본 설정 및 환경 구성 확인 필요"
    fi
fi

if [[ $success_rate -lt 90 ]]; then
    echo "  📚 문서: 실패 케이스에 대한 사용 가이드 보완"
    echo "  🔧 코드: 에러 처리 및 사용자 피드백 개선"
    echo "  🧪 테스트: 실패 케이스에 대한 단위 테스트 추가"
fi

echo
echo "📝 다음 단계"
echo "============="
echo "1. 실패한 테스트 개별 검토 및 수정"
echo "2. 개선된 버전으로 재테스트 실행"
echo "3. 성공률 90% 이상 달성 시 배포 고려"
echo "4. 사용자 피드백 수집 및 반영"

echo
echo "📁 관련 파일"
echo "============="
echo "QA 결과: $QA_RESULTS_FILE"
echo "QA 가이드: FUNCTIONAL_QA_GUIDE.md"
echo "테스트 스크립트: run_functional_qa.sh"

# HTML 리포트 생성 (선택적)
if command -v python3 &> /dev/null; then
    HTML_REPORT="${QA_RESULTS_FILE%.json}.html"
    
    python3 << EOF
import json
import sys
from datetime import datetime

try:
    with open('$QA_RESULTS_FILE', 'r') as f:
        data = json.load(f)
    
    html_content = '''
<!DOCTYPE html>
<html>
<head>
    <title>DevOrch QA Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .pass { color: green; }
        .fail { color: red; }
        .summary { background: #f5f5f5; padding: 15px; border-radius: 5px; }
        .test-item { margin: 10px 0; padding: 10px; border-left: 3px solid #ccc; }
        .test-pass { border-left-color: green; }
        .test-fail { border-left-color: red; }
    </style>
</head>
<body>
    <h1>DevOrch QA Test Report</h1>
    <div class="summary">
        <h2>Summary</h2>
        <p><strong>Total Tests:</strong> $total_tests</p>
        <p><strong>Passed:</strong> <span class="pass">$passed_tests</span></p>
        <p><strong>Failed:</strong> <span class="fail">$failed_tests</span></p>
        <p><strong>Success Rate:</strong> $success_rate%</p>
        <p><strong>Generated:</strong> $(date)</p>
    </div>
    
    <h2>Test Results</h2>
'''
    
    for test in data['test_results']:
        status_class = 'test-pass' if test['result'] == 'PASS' else 'test-fail'
        status_color = 'pass' if test['result'] == 'PASS' else 'fail'
        
        html_content += f'''
    <div class="test-item {status_class}">
        <h3>{test['test_name']} - <span class="{status_color}">{test['result']}</span></h3>
        <p><strong>Description:</strong> {test['description']}</p>
        <p><strong>Exit Code:</strong> {test['exit_code']}</p>
        <p><strong>Timestamp:</strong> {test['timestamp']}</p>
'''
        if test.get('details'):
            html_content += f'        <p><strong>Details:</strong> {test["details"]}</p>'
        
        html_content += '    </div>'
    
    html_content += '''
</body>
</html>'''
    
    with open('$HTML_REPORT', 'w') as f:
        f.write(html_content)
    
    print(f"HTML 리포트 생성: $HTML_REPORT")
    
except Exception as e:
    print(f"HTML 리포트 생성 실패: {e}")
EOF
fi

echo
echo "분석 완료! 🎯"