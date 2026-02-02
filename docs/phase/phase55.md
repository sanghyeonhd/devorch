# Phase 55-58: Web UI

## 개요
웹 기반 사용자 인터페이스를 구현했습니다. SSE(Server-Sent Events)를 사용한 실시간 스트리밍, 세션 관리, 설정 기능을 제공합니다.

## 구현 파일

### internal/webui/server.go
**Server 구조:**
```go
type Server struct {
    httpServer  *http.Server
    mux         *http.ServeMux
    sseClients  map[string]chan SSEEvent
    handlers    map[string]APIHandler
    middlewares []Middleware
}
```

**Config:**
```go
type Config struct {
    Addr           string        // ":8080"
    ReadTimeout    time.Duration // 30s
    WriteTimeout   time.Duration // 30s
    MaxHeaderBytes int           // 1MB
    EnableCORS     bool          // true
    CORSOrigins    []string      // ["*"]
}
```

**주요 기능:**
- 정적 파일 서빙 (embed.FS 사용)
- API 핸들러 등록
- SSE 클라이언트 관리
- CORS 미들웨어
- 브로드캐스트/개별 전송

**메서드:**
```go
server := webui.NewServer(webui.DefaultConfig())

// 핸들러 등록
server.RegisterHandler("/sessions", sessionsHandler)

// 미들웨어 추가
server.AddMiddleware(authMiddleware)

// 시작
go server.Start()

// SSE 브로드캐스트
server.Broadcast(webui.SSEEvent{
    Event: "message",
    Data:  map[string]any{"content": "Hello"},
})

// 특정 클라이언트에 전송
server.SendTo(clientID, event)

// 종료
server.Shutdown(ctx)
```

### internal/webui/handlers.go
**ChatService:**
```go
type ChatService struct {
    sessions map[string]*ChatSession
    server   *Server
}
```

**API 엔드포인트:**
| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | /api/sessions | 세션 목록 |
| POST | /api/sessions | 새 세션 생성 |
| POST | /api/chat | 메시지 전송 |
| GET | /events | SSE 연결 |
| GET | /health | 헬스 체크 |

### internal/webui/static/
**index.html:**
- 반응형 레이아웃
- 사이드바 (세션 목록, 설정)
- 채팅 영역
- 입력 영역
- 상태 표시줄

**styles.css:**
- CSS 변수 기반 테마
- 다크 모드 기본
- 반응형 디자인 (768px 브레이크포인트)
- 커스텀 스크롤바

**app.js:**
- SSE 연결 관리
- 세션 CRUD
- 메시지 전송/수신
- 마크다운 렌더링
- 설정 관리

## SSE (Server-Sent Events)
실시간 양방향 통신:

**클라이언트 연결:**
```javascript
const eventSource = new EventSource('/events?client_id=' + clientId);

eventSource.addEventListener('connected', (e) => {
    console.log('Connected:', JSON.parse(e.data));
});

eventSource.addEventListener('message', (e) => {
    const data = JSON.parse(e.data);
    handleMessage(data);
});

eventSource.addEventListener('stream', (e) => {
    const data = JSON.parse(e.data);
    handleStreamChunk(data);
});
```

**서버 브로드캐스트:**
```go
server.Broadcast(webui.SSEEvent{
    Event: "stream",
    Data: map[string]any{
        "content": "Hello",
        "done":    false,
    },
})
```

## API 요청/응답
**세션 생성:**
```http
POST /api/sessions
Content-Type: application/json

{"name": "My Session"}
```

**채팅:**
```http
POST /api/chat
Content-Type: application/json

{
    "session_id": "abc123",
    "message": "Hello",
    "model": "claude-sonnet-4-20250514",
    "temperature": 0.7
}
```

**응답:**
```json
{
    "session_id": "abc123",
    "message": {
        "id": "msg123",
        "role": "assistant",
        "content": "Hello! How can I help?",
        "created_at": "2025-01-01T10:00:00Z"
    }
}
```

## 테마 커스터마이징
CSS 변수로 테마 변경:
```css
:root {
    --bg-primary: #1a1a2e;
    --bg-secondary: #16213e;
    --text-primary: #e8e8e8;
    --accent-primary: #e94560;
    /* ... */
}
```

## 사용 예시
```go
import "devorch/internal/webui"

// 서버 생성
config := webui.DefaultConfig()
config.Addr = ":3000"
server := webui.NewServer(config)

// 채팅 서비스 연결
chatService := webui.NewChatService(server)

// 시작
if err := server.Start(); err != nil {
    log.Fatal(err)
}
```

## 기능 목록
- [x] 세션 관리 (생성, 조회, 선택)
- [x] 실시간 채팅
- [x] SSE 스트리밍
- [x] 모델 선택
- [x] Temperature 조절
- [x] 마크다운 렌더링
- [x] 코드 하이라이팅
- [x] 반응형 디자인
- [x] 다크 모드
- [x] 사이드바 토글

## 향후 개선
- [ ] WebSocket 지원 (양방향 통신)
- [ ] 파일 업로드
- [ ] 이미지 표시
- [ ] 도구 실행 결과 시각화
- [ ] 키보드 단축키
- [ ] 다국어 지원
