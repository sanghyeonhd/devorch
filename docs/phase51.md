# Phase 51-52: Session Management Enhancement

## 개요
세션 관리 기능을 확장하여 컨텍스트 압축(compaction), 포크, 내보내기/가져오기, 검색 기능을 추가했습니다.

## 구현 파일

### internal/session/session.go
**Message 구조:**
```go
type Message struct {
    ID          string
    Role        MessageRole  // user, assistant, system, tool
    Content     string
    Tokens      int
    ToolCalls   []ToolCall
    ToolResults []ToolResult
    Metadata    map[string]any
    CreatedAt   time.Time
}
```

**Session 구조:**
```go
type Session struct {
    ID          string
    Name        string
    Workspace   string
    Messages    []Message
    Metadata    map[string]any
    Summary     string
    Tags        []string
    ParentID    string    // 포크된 세션의 부모
    CreatedAt   time.Time
    UpdatedAt   time.Time
    TotalTokens int
}
```

**주요 메서드:**
- `NewSession()`: 새 세션 생성
- `AddMessage()`: 메시지 추가
- `GetMessages()`: 모든 메시지 조회
- `GetRecentMessages(n)`: 최근 n개 메시지
- `Fork(newID, newName, fromIdx)`: 세션 포크
- `ToJSON() / FromJSON()`: JSON 직렬화

**Compaction (컨텍스트 압축):**
```go
type CompactionStrategy int
const (
    CompactByRemovingOld          // 오래된 메시지 제거
    CompactBySummarizing          // 요약으로 대체
    CompactByRemovingToolResults  // 도구 결과 축소
)

opts := session.DefaultCompactOptions()
opts.KeepRecent = 10
opts.TargetTokens = 4000

session.Compact(ctx, opts, summarizerFunc)
```

### internal/session/store.go
**Store 기능:**
- `NewStore(dir)`: 스토어 생성
- `Create(ctx, id, name, workspace)`: 세션 생성
- `Get(ctx, id)`: 세션 조회
- `Update(ctx, session)`: 세션 업데이트
- `Delete(ctx, id)`: 세션 삭제
- `List(ctx, workspace)`: 세션 목록
- `Fork(ctx, sourceID, newID, name, fromIdx)`: 포크
- `Search(ctx, workspace, query)`: 검색

**Import/Export:**
```go
// JSON 또는 Markdown으로 내보내기
store.Export(ctx, sessionID, "json", "/path/export.json")
store.Export(ctx, sessionID, "markdown", "/path/export.md")

// 가져오기
session, err := store.Import(ctx, "/path/session.json")
```

**통계:**
```go
stats, err := store.Stats(ctx, workspace)
// SessionStats{
//     TotalSessions: 10,
//     TotalMessages: 150,
//     TotalTokens: 45000,
//     AvgMessagesPerSession: 15.0,
//     OldestSession: time.Time{...},
//     NewestSession: time.Time{...},
// }
```

**자동 저장:**
```go
config := session.AutoSaveConfig{
    Enabled:  true,
    Interval: 5 * time.Minute,
}
store.StartAutoSave(ctx, config)
```

**백업/복원:**
```go
// 전체 백업
store.Backup(ctx, "/path/backup.json")

// 복원 (overwrite=false면 기존 세션 유지)
store.Restore(ctx, "/path/backup.json", false)
```

## Compaction 전략

### 1. CompactByRemovingOld
가장 단순한 전략으로, 오래된 메시지를 제거하고 최근 메시지만 유지합니다.
- 시스템 메시지는 항상 보존
- 지정된 개수의 최근 메시지만 유지

### 2. CompactBySummarizing
오래된 메시지를 요약하여 하나의 시스템 메시지로 대체합니다.
- LLM을 사용하여 요약 생성
- 요약 실패 시 RemovingOld로 폴백

### 3. CompactByRemovingToolResults
도구 실행 결과를 축소하여 토큰 사용량을 줄입니다.
- 긴 도구 결과를 500자로 제한
- 전체 메시지 구조는 유지

## 사용 예시
```go
import "devorch/internal/session"

// 스토어 생성
store, err := session.NewStore("/data/sessions")

// 세션 생성
sess, err := store.Create(ctx, "sess-1", "My Session", "workspace-1")

// 메시지 추가
sess.AddMessage(session.Message{
    Role:    session.RoleUser,
    Content: "Hello",
    Tokens:  2,
})

// 저장
store.Update(ctx, sess)

// 포크
fork, err := store.Fork(ctx, "sess-1", "sess-2", "Forked Session", 5)

// 검색
results, err := store.Search(ctx, "workspace-1", "error handling")

// 내보내기
store.Export(ctx, "sess-1", "markdown", "session.md")
```

## Markdown 내보내기 형식
```markdown
# My Session

**Created:** 2025-01-01T10:00:00Z
**Updated:** 2025-01-01T12:00:00Z

**Tags:** coding, debug

---

### 👤 User

Hello, can you help me?

---

### 🤖 Assistant

Of course! What do you need help with?

---
```
