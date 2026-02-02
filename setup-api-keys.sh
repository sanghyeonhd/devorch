#!/bin/bash

# DevOrch API Keys 환경변수 설정 스크립트
# 사용법: source ./setup-api-keys.sh

# 제공받은 API 키들을 환경변수로 설정
export OPENAI_API_KEY="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"

export ANTHROPIC_API_KEY="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"

export GEMINI_API_KEY="AIzaSyCVBgrsgtGNro9tX89POR_hXjDHrXoMQSc"

export OPENROUTER_API_KEY="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"

# Google AI API Key (Gemini용)
export GOOGLE_API_KEY="$GEMINI_API_KEY"

# 설정 완료 메시지
echo "✅ DevOrch API Keys 설정 완료:"
echo "   - OpenAI: ${OPENAI_API_KEY:0:10}...${OPENAI_API_KEY: -10}"
echo "   - Anthropic: ${ANTHROPIC_API_KEY:0:15}...${ANTHROPIC_API_KEY: -10}"
echo "   - Gemini: ${GEMINI_API_KEY:0:10}...${GEMINI_API_KEY: -5}"
echo "   - OpenRouter: ${OPENROUTER_API_KEY:0:15}...${OPENROUTER_API_KEY: -10}"
echo ""
echo "이제 './bin/devorch doctor'로 시스템 상태를 확인할 수 있습니다."

# 영구 설정을 위해 .bashrc 또는 .zshrc에 추가하는 함수
setup_persistent_keys() {
    local shell_config=""
    
    if [ -n "$ZSH_VERSION" ]; then
        shell_config="$HOME/.zshrc"
    elif [ -n "$BASH_VERSION" ]; then
        shell_config="$HOME/.bashrc"
    fi
    
    if [ -n "$shell_config" ]; then
        echo ""
        echo "영구 설정을 위해 $shell_config에 API 키를 추가하시겠습니까? (y/n)"
        read -r response
        if [ "$response" = "y" ] || [ "$response" = "Y" ]; then
            echo "" >> "$shell_config"
            echo "# DevOrch AI OS API Keys" >> "$shell_config"
            echo "export OPENAI_API_KEY=\"$OPENAI_API_KEY\"" >> "$shell_config"
            echo "export ANTHROPIC_API_KEY=\"$ANTHROPIC_API_KEY\"" >> "$shell_config"
            echo "export GEMINI_API_KEY=\"$GEMINI_API_KEY\"" >> "$shell_config"
            echo "export GOOGLE_API_KEY=\"$GEMINI_API_KEY\"" >> "$shell_config"
            echo "export OPENROUTER_API_KEY=\"$OPENROUTER_API_KEY\"" >> "$shell_config"
            echo ""
            echo "✅ $shell_config에 API 키가 추가되었습니다."
            echo "새 터미널에서는 자동으로 로드됩니다."
        fi
    fi
}

# 함수 실행 (interactive 모드에서만)
if [ -t 0 ]; then
    setup_persistent_keys
fi