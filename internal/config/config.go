package config

type LocalRuntimeConfig struct {
	OfflineMode      bool   // 폐쇄망/오프라인 모드 (다운로드 제한)
	AllowAutoInstall bool   // 런타임 자동 설치 허용
	RuntimeDir       string // 런타임 설치 위치 (없으면 global.RuntimeDir())
	ModelsDir        string // 로컬 모델/매니페스트 저장
}

type OllamaConfig struct {
	Host           string // 기본: http://127.0.0.1:11434
	AutoStart      bool   // 번들 ollama 자동 실행
	BundleOverride string // 번들 바이너리 경로 오버라이드 (디버그)
}

type Config struct {
	DBPath     string
	DaemonAddr string

	OpenAIKey     string
	OpenRouterKey string

	Local  LocalRuntimeConfig
	Ollama OllamaConfig
}
