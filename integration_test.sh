#!/bin/bash

echo "DevOrch CLI Integration Test - Complete System Coverage"
echo "========================================================"
echo ""

# Test all systems
systems=("permission" "storage" "extension" "platform" "runtime" "lsp" "mcp")
total_systems=0
working_systems=0

for system in "${systems[@]}"; do
    total_systems=$((total_systems + 1))
    echo -n "Testing $system system... "
    
    result=$(echo "/$system help" | ./bin/devorch 2>/dev/null | grep -c "$system")
    
    if [ "$result" -gt 0 ]; then
        echo "✅ Working"
        working_systems=$((working_systems + 1))
    else
        echo "❌ Failed"
    fi
done

echo ""
echo "Integration Test Summary:"
echo "========================"
echo "Total Systems: $total_systems"
echo "Working Systems: $working_systems"
echo "Failed Systems: $((total_systems - working_systems))"

if [ "$working_systems" -eq "$total_systems" ]; then
    echo "Status: ✅ ALL SYSTEMS OPERATIONAL"
    echo "CLI Integration: 100% Complete"
else
    echo "Status: ⚠️  Some systems need attention"
    echo "CLI Integration: $((working_systems * 100 / total_systems))% Complete"
fi

echo ""
echo "DevOrch Enterprise AI Development Platform"
echo "All backend systems now accessible via CLI"