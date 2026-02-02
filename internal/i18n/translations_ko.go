package i18n

// Korean translations
var koreanTranslations = map[string]string{
	// General
	"app.name":        "DevOrch",
	"app.description": "AI 기반 개발 오케스트레이션 도구",
	"app.version":     "버전 %s",

	// Commands
	"cmd.help":    "도움말 표시",
	"cmd.version": "버전 표시",
	"cmd.config":  "설정 구성",
	"cmd.chat":    "채팅 세션 시작",
	"cmd.run":     "작업 실행",
	"cmd.init":    "프로젝트 초기화",

	// Chat
	"chat.welcome":     "DevOrch에 오신 것을 환영합니다! 메시지를 입력하거나 /help로 명령어를 확인하세요.",
	"chat.thinking":    "생각 중...",
	"chat.typing":      "입력 중...",
	"chat.error":       "오류: %s",
	"chat.exit":        "안녕히 가세요!",
	"chat.clear":       "채팅이 지워졌습니다.",
	"chat.saved":       "채팅이 %s에 저장되었습니다",
	"chat.loaded":      "채팅이 %s에서 로드되었습니다",
	"chat.new_session": "새 세션을 시작합니다...",

	// Tools
	"tool.executing":  "%s 실행 중...",
	"tool.completed":  "%s 완료",
	"tool.failed":     "실패: %s",
	"tool.permission": "%s에 대한 권한이 필요합니다",
	"tool.approve":    "승인",
	"tool.deny":       "거부",
	"tool.always":     "항상 허용",

	// Files
	"file.reading":  "%s 읽는 중...",
	"file.writing":  "%s 쓰는 중...",
	"file.creating": "%s 생성 중...",
	"file.deleting": "%s 삭제 중...",
	"file.modified": "파일 수정됨: %s",
	"file.created":  "파일 생성됨: %s",
	"file.deleted":  "파일 삭제됨: %s",
	"file.notfound": "파일을 찾을 수 없음: %s",

	// Git
	"git.staging":    "변경사항 스테이징 중...",
	"git.committing": "커밋 중...",
	"git.pushing":    "푸시 중...",
	"git.pulling":    "풀 중...",
	"git.status":     "Git 상태",
	"git.diff":       "Git 차이점",

	// Providers
	"provider.connecting": "%s에 연결 중...",
	"provider.connected":  "%s에 연결됨",
	"provider.error":      "프로바이더 오류: %s",
	"provider.ratelimit":  "요청 제한에 도달했습니다. 대기 중...",
	"provider.retry":      "%d초 후 재시도...",

	// Settings
	"settings.saved":   "설정이 저장되었습니다",
	"settings.loaded":  "설정이 로드되었습니다",
	"settings.reset":   "설정이 기본값으로 초기화되었습니다",
	"settings.invalid": "잘못된 설정: %s",

	// Errors
	"error.generic":     "오류가 발생했습니다: %s",
	"error.network":     "네트워크 오류: %s",
	"error.permission":  "권한 거부: %s",
	"error.notfound":    "찾을 수 없음: %s",
	"error.invalid":     "잘못된 입력: %s",
	"error.timeout":     "작업 시간 초과",
	"error.interrupted": "작업이 중단되었습니다",

	// Confirmations
	"confirm.yes":     "예",
	"confirm.no":      "아니오",
	"confirm.cancel":  "취소",
	"confirm.proceed": "진행하시겠습니까?",
	"confirm.delete":  "%s을(를) 삭제하시겠습니까?",
	"confirm.reset":   "초기화하시겠습니까?",

	// Status
	"status.ready":      "준비됨",
	"status.busy":       "작업 중",
	"status.waiting":    "대기 중...",
	"status.processing": "처리 중...",
	"status.complete":   "완료",
	"status.failed":     "실패",

	// Terminal Interface
	"terminal.input.placeholder": "메시지를 입력하세요...",

	// CLI Interface
	"cli.title":    "DevOrch CLI",
	"cli.settings": "설정",
	"cli.history":  "기록",
	"cli.new_chat": "새 채팅",
	"cli.export":   "내보내기",
	"cli.import":   "가져오기",

	// Memory
	"memory.title":    "프로젝트 메모리",
	"memory.saved":    "메모리가 저장되었습니다",
	"memory.loaded":   "메모리가 로드되었습니다",
	"memory.cleared":  "메모리가 지워졌습니다",
	"memory.notfound": "메모리를 찾을 수 없습니다",

	// Session
	"session.new":       "새 세션이 생성되었습니다",
	"session.restored":  "세션이 복원되었습니다",
	"session.compacted": "세션이 압축되었습니다: %d개 메시지",
	"session.expired":   "세션이 만료되었습니다",
}
