#!/bin/bash

# DevOrch CLI 완전 통합 테스트 스크립트
echo "🎯 DevOrch CLI 완전 통합 테스트 시작"
echo "======================================"

DEVORCH_CLI="./bin/devorch --cli"

# 새로 추가된 고급 기능 명령어들 테스트
COMMANDS=(
    "/help"
    "/learn help"
    "/analytics help"
    "/quality help"
    "/benchmark help"
    "/hardware help"
    "/autosetup help"
    "/install help"
    "/lsp help"
    "/editor help"
    "/terminal help"
    "/shell help"
    "/workspace help"
    "/history help"
    "/plugin help"
    "/experiment help"
    "/websocket help"
    "/mcp help"
    "/api help"
    "/events help"
    "/billing help"
    "/budget help"
    "/usage help"
    "/doctor help"
    "/logs help"
    "/metrics help"
    "/debug help"
    "/i18n help"
    "/theme help"
    "/memory help"
    "/file help"
    "/task help"
    "/delegate help"
)

success_count=0
fail_count=0

for cmd in "${COMMANDS[@]}"; do
    echo -n "Testing: $cmd ... "
    if echo "$cmd" | timeout 5s $DEVORCH_CLI > /dev/null 2>&1; then
        echo "✅ OK"
        ((success_count++))
    else
        echo "❌ FAIL"
        ((fail_count++))
    fi
done

echo ""
echo "======================================"
echo "📊 테스트 결과:"
echo "  ✅ 성공: $success_count 개"
echo "  ❌ 실패: $fail_count 개"
echo "  📈 성공률: $(echo "scale=1; $success_count * 100 / ($success_count + $fail_count)" | bc)%"

if [ $fail_count -eq 0 ]; then
    echo "🎉 모든 CLI 명령어가 완전히 통합되었습니다!"
else
    echo "⚠️  일부 명령어에서 문제가 발견되었습니다."
fi

echo "======================================"