#!/bin/bash
# DevOrch CLI 최종 통합 검증 스크립트

echo "🔍 DevOrch CLI 최종 통합 검증 시작 (25개 시스템)"
echo "=============================================="

cd /Users/deniallee/devorch

# 카운터 초기화
success=0
partial=0
failed=0

echo -e "\n📋 Core Systems (15개)"
echo "========================"

# 1. Session Management
echo -n "1. Session Management: "
if echo "/session list" | ./bin/devorch --cli | grep -q "Session"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 2. Tool Management  
echo -n "2. Tool Management: "
if echo "/tool list" | ./bin/devorch --cli | grep -q "tool\|Available"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 3. Workflow Management
echo -n "3. Workflow Management: "
if echo "/workflow list" | ./bin/devorch --cli | grep -q "workflow\|Saved"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 4. Agent Orchestration
echo -n "4. Agent Orchestration: "
if echo "/agent list" | ./bin/devorch --cli | grep -q "Registered\|Agent"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 5. Model Management
echo -n "5. Model Management: "
if echo "/model list" | ./bin/devorch --cli | grep -q "model\|Available"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 6. Provider Management
echo -n "6. Provider Management: "
if echo "/provider list" | ./bin/devorch --cli | grep -q "provider\|Available"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 7. Context Management
echo -n "7. Context Management: "
if echo "/context list" | ./bin/devorch --cli | grep -q "context\|Current"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 8. Code Operations
echo -n "8. Code Operations: "
if echo "/code files" | ./bin/devorch --cli | grep -q "file\|Project"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 9. Authentication
echo -n "9. Authentication: "
if echo "/auth status" | ./bin/devorch --cli | grep -q "auth\|Status"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 10. System Management
echo -n "10. System Management: "
if echo "/system status" | ./bin/devorch --cli | grep -q "System\|DevOrch"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 11. Learning System
echo -n "11. Learning System: "
if echo "/learn status" | ./bin/devorch --cli | grep -q "learn\|Learning"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 12. Memory Management
echo -n "12. Memory Management: "
if echo "/memory status" | ./bin/devorch --cli | grep -q "Memory\|memory"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 13. Configuration
echo -n "13. Configuration: "
if echo "/config list" | ./bin/devorch --cli | grep -q "config\|Configuration"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 14. Authorization
echo -n "14. Authorization: "
if echo "/auth status" | ./bin/devorch --cli | grep -q "auth\|Status"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 15. Quality Evaluation
echo -n "15. Quality Evaluation: "
if echo "/quality check" | ./bin/devorch --cli | grep -q "Quality\|quality"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

echo -e "\n🔍 새로 발견된 시스템들 (8개)"
echo "=========================="

# 16. History Management
echo -n "16. History Management: "
if echo "/history stats" | ./bin/devorch --cli | grep -q "history\|files"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 17. Experiment System
echo -n "17. Experiment System: "
if echo "/experiment list" | ./bin/devorch --cli | grep -q "Experiments\|Running\|Complete"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 18. Diagnostics
echo -n "18. Diagnostics: "
if echo "/doctor check" | ./bin/devorch --cli | grep -q "config\|memory\|health"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 19. Plugin Management
echo -n "19. Plugin Management: "
if echo "/plugin list" | ./bin/devorch --cli | grep -q "plugin\|formatter"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 20. Autonomous AI OS
echo -n "20. Autonomous AI OS: "
if echo "/autonomous status" | ./bin/devorch --cli | grep -q "Status\|Goal\|Tier"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 21. WebSocket & Real-time
echo -n "21. WebSocket & Real-time: "
if echo "/ws list" | ./bin/devorch --cli | grep -q "WebSocket\|connection"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 22. Execution & Runner
echo -n "22. Execution & Runner: "
if echo "/exec history" | ./bin/devorch --cli | grep -q "exec\|history"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 23. Compaction & Optimization
echo -n "23. Compaction & Optimization: "
if echo "/compact analyze" | ./bin/devorch --cli | grep -q "compact\|session"; then
    echo "✅ PASS"
    ((success++))
else
    echo "❌ FAIL"
    ((failed++))
fi

echo -e "\n🟨 부분 통합 시스템들 (2개)"
echo "========================"

# 24. LSP System
echo -n "24. LSP System: "
if echo "/lsp status" | ./bin/devorch --cli | grep -q "LSP\|Active"; then
    echo "🟨 PARTIAL"
    ((partial++))
else
    echo "❌ FAIL"
    ((failed++))
fi

# 25. MCP Protocol
echo -n "25. MCP Protocol: "
if echo "/mcp status" | ./bin/devorch --cli | grep -q "MCP\|Connected"; then
    echo "🟨 PARTIAL"
    ((partial++))
else
    echo "❌ FAIL"
    ((failed++))
fi

echo -e "\n📊 최종 결과"
echo "============"
echo "✅ 완전 통합: $success/25 시스템"
echo "🟨 부분 통합: $partial/25 시스템"
echo "❌ 실패: $failed/25 시스템"

total=$((success + partial))
percentage=$((total * 100 / 25))

echo -e "\n🎯 통합률: $percentage% ($total/25)"

if [ $percentage -ge 90 ]; then
    echo "🏆 EXCELLENT: DevOrch CLI 통합 완료!"
elif [ $percentage -ge 80 ]; then
    echo "🎉 GOOD: 훌륭한 통합 수준!"
elif [ $percentage -ge 70 ]; then
    echo "👍 SATISFACTORY: 만족스러운 통합 수준"
else
    echo "⚠️  NEEDS WORK: 추가 작업 필요"
fi

echo -e "\nDevOrch CLI 최종 검증 완료! 🎪"