#!/bin/bash

# Test connected providers with simple questions

echo "Testing DevOrch Providers..."

# Function to test provider
test_provider() {
    local provider=$1
    local model=$2
    local timeout_duration=${3:-10}
    
    echo ""
    echo "🔵 Testing $provider ($model)..."
    echo "================================================="
    
    # Test with a simple question
    echo "Question: What is 2+2?"
    
    local test_result=""
    test_result=$(timeout ${timeout_duration}s bash -c "echo '/provider $provider
/model $model
What is 2+2? Please answer briefly.' | ./devorch" 2>/dev/null)
    
    if [[ $? -eq 0 ]]; then
        echo "✅ $provider responds!"
        echo "Response snippet:"
        echo "$test_result" | tail -5
    else
        echo "❌ $provider failed or timeout"
    fi
    
    echo "================================================="
    echo ""
}

# Test connected providers
test_provider "github-copilot" "gpt-4o" 15
test_provider "openai" "gpt-4o" 15
test_provider "google" "gemini-2.0-flash" 15
test_provider "ollama" "qwen2.5:7b" 10

echo "Testing completed!"