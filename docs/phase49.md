# Phase 49-50: LSP (Language Server Protocol) Integration

## 개요
Language Server Protocol 클라이언트를 구현하여 코드 인텔리전스 기능을 AI 도구에 통합했습니다.

## 구현 파일

### internal/lsp/client.go
- **Client**: LSP 클라이언트
- 언어 서버 프로세스 관리
- JSON-RPC 2.0 통신
- 요청/응답 매칭
- 초기화 및 종료 처리

주요 메서드:
- `NewClient(command, args...)`: 클라이언트 생성
- `Start(ctx)`: 서버 프로세스 시작
- `Initialize(ctx, rootURI)`: LSP 초기화
- `Shutdown(ctx)`: 서버 종료
- `Close(ctx)`: 연결 종료

### internal/lsp/types.go
LSP 프로토콜 타입 정의:

**기본 타입:**
- `Position`: 텍스트 위치 (line, character)
- `Range`: 텍스트 범위
- `Location`: URI + Range
- `LocationLink`: 정의 위치 링크

**문서 관련:**
- `TextDocumentIdentifier`: 문서 식별자
- `TextDocumentItem`: 문서 전송용
- `TextDocumentPositionParams`: 문서+위치

**Capabilities:**
- `ClientCapabilities`: 클라이언트 기능
- `ServerCapabilities`: 서버 기능

**결과 타입:**
- `Hover`: 호버 정보
- `CompletionList/Item`: 자동완성
- `Diagnostic`: 진단 정보
- `DocumentSymbol`: 문서 심볼
- `SymbolInformation`: 심볼 정보

### internal/lsp/operations.go
LSP 오퍼레이션 구현:

**문서 동기화:**
- `TextDocumentDidOpen()`: 문서 열기
- `TextDocumentDidClose()`: 문서 닫기
- `TextDocumentDidChange()`: 문서 변경

**코드 인텔리전스:**
- `GotoDefinition()`: 정의로 이동
- `FindReferences()`: 참조 찾기
- `Hover()`: 호버 정보
- `Completion()`: 자동완성
- `DocumentSymbols()`: 문서 심볼
- `WorkspaceSymbols()`: 워크스페이스 심볼

**코드 편집:**
- `Formatting()`: 포맷팅
- `Rename()`: 이름 변경
- `CodeAction()`: 코드 액션

### internal/lsp/manager.go
여러 언어 서버 관리:

- **Manager**: 멀티 언어 서버 매니저
- **ServerConfig**: 서버 설정
- **DefaultConfigs**: 기본 설정 (Go, TypeScript, Python, Rust, C/C++)

유틸리티:
- `GetLanguageForFile(path)`: 파일 확장자로 언어 판별
- `FileToURI(path)`: 파일 경로 → URI
- `URIToFile(uri)`: URI → 파일 경로

## 지원 언어 서버
| 언어 | 서버 |
|---|---|
| Go | gopls |
| TypeScript/JS | typescript-language-server |
| Python | pylsp |
| Rust | rust-analyzer |
| C/C++ | clangd |

## 사용 예시
```go
import "devorch/internal/lsp"

// 매니저 생성
manager := lsp.NewManager()

// 클라이언트 가져오기
client, err := manager.GetClient(ctx, "go", "file:///project")
if err != nil {
    log.Fatal(err)
}

// 정의로 이동
locations, err := client.GotoDefinition(ctx, 
    "file:///project/main.go", 
    10, // line
    5,  // character
)

// 참조 찾기
refs, err := client.FindReferences(ctx,
    "file:///project/main.go",
    10, 5,
    true, // includeDeclaration
)

// 자동완성
completions, err := client.Completion(ctx,
    "file:///project/main.go",
    10, 5,
)

// 종료
manager.CloseAll(ctx)
```

## AI 도구 통합
LSP 기능은 AI 도구로 노출되어 LLM이 코드베이스를 탐색할 수 있습니다:

```go
// Tool: goto_definition
// 심볼의 정의 위치를 찾습니다
{
    "file": "/project/main.go",
    "line": 10,
    "character": 5
}

// Tool: find_references
// 심볼의 모든 참조를 찾습니다
{
    "file": "/project/main.go",
    "line": 10,
    "character": 5,
    "include_declaration": true
}

// Tool: document_symbols
// 파일의 모든 심볼을 나열합니다
{
    "file": "/project/main.go"
}
```
