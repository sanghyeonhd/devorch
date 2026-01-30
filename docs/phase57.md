# Phase 57: OAuth 인증 시스템 구현 (OpenCode 수준)

## 개요
OpenCode에서 제공하는 `/connect` 기능과 동일한 수준의 OAuth 인증 시스템을 구현했습니다.
API 키가 아닌 **구독형 모델(ChatGPT Pro/Plus, Claude Pro/Max, GitHub Copilot)의 인증 토큰**을 연동하여 사용할 수 있습니다.

## 지원하는 OAuth 인증 (3가지)

### 1. OpenAI - ChatGPT Pro/Plus (PKCE OAuth + Auto Callback)
```
https://auth.openai.com/oauth/authorize?
  response_type=code&
  client_id=app_EMoamEEZ73f0CkXaXp7hrann&
  redirect_uri=http://localhost:1455/auth/callback&
  scope=openid+profile+email+offline_access&
  code_challenge=...&
  code_challenge_method=S256&
  id_token_add_organizations=true&
  codex_cli_simplified_flow=true&
  originator=opencode

Waiting for authorization…
```
- **방식**: 브라우저 OAuth → localhost:1455 자동 콜백
- **결과**: Access Token 자동 획득

### 2. GitHub Copilot (Device Code Flow)
```
https://github.com/login/device
Enter code: 6CA7-C57F

Waiting for authorization…
```
- **방식**: Device Code Flow (폴링)
- **결과**: GitHub OAuth Token 획득

### 3. Anthropic - Claude Pro/Max (PKCE OAuth + Manual Code)
```
https://claude.ai/oauth/authorize?
  code=true&
  client_id=9d1c250a-e61b-44d9-88ed-5944d1962f5e&
  response_type=code&
  redirect_uri=https://console.anthropic.com/oauth/code/callback&
  scope=org:create_api_key+user:profile+user:inference&
  code_challenge=...&
  code_challenge_method=S256

Waiting for authorization…
```
- **방식**: 브라우저 인증 후 코드 수동 복사/붙여넣기
- **결과**: API 키 생성 권한이 있는 Access Token

## 사용법

### CLI 모드에서 OAuth 연결
```bash
devorch
❯ /connect

# 프로바이더 선택
1. ○ Anthropic
2. ○ GitHub Copilot
3. ○ OpenAI
...

# 인증 방식 선택
1. Claude Pro/Max - Login with Claude subscription (OAuth)
2. Manually enter API Key - Get API key from console.anthropic.com
```

### 각 프로바이더별 OAuth 흐름

#### OpenAI (ChatGPT Pro/Plus)
```
Starting OAuth authorization...

────────────────────────────────────────

  https://auth.openai.com/oauth/authorize?...

────────────────────────────────────────

Waiting for authorization…

✓ Authorization successful!
  Connected to: OpenAI
```

#### GitHub Copilot
```
Starting device authorization...

────────────────────────────────────────

  https://github.com/login/device
  Enter code: 3E89-268A

────────────────────────────────────────

Waiting for authorization…

✓ Authorization successful!
  Connected to: GitHub Copilot
```

#### Anthropic (Claude Pro/Max)
```
Starting OAuth authorization...

────────────────────────────────────────

  https://claude.ai/oauth/authorize?...

────────────────────────────────────────

Waiting for authorization…

After authorization, copy the code from the browser and paste it here:

Code: ████████████

✓ Authorization successful!
  Connected to: Anthropic
```

## 파일 구조
```
internal/
  auth/
    auth.go          # OAuth 인증 시스템 (PKCE, Device Flow, Callback Server)
  tui/
    cli_mode.go      # CLI 모드 OAuth UI
    provider.go      # 프로바이더 정의
```

## OpenCode와의 완전 호환

| 기능 | OpenCode | DevOrch |
|------|----------|---------|
| OpenAI PKCE OAuth | ✅ | ✅ |
| GitHub Device Code Flow | ✅ | ✅ |
| Anthropic PKCE + code=true | ✅ | ✅ |
| localhost 콜백 서버 | ✅ | ✅ |
| 브라우저 자동 오픈 | ✅ | ✅ |
| 토큰 저장 (auth.json) | ✅ | ✅ |
