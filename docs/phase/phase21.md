좋아요. 21단계로 진행하겠습니다.

이번 단계 목표는:

1단계(현재까지) 소스트리 대비 누락된 “개발 항목” 점검

그리고 곧바로 OpenCode/oh-my-opencode 방식의 설정 시스템(JSONC + 머지 우선순위 + 소스 체인 + validate) 를 “풀코드”로 추가하는 것입니다.
(= 당신이 요구한 “프로젝트별 override / 전역 / env / inline / 추가 config dir / .devorch” 체인을 실제로 동작하게 만듭니다.)



---

A) 1단계 기준 “소스트리 대비 누락된 개발 항목” (핵심만 체크)

아래는 당신이 제시한 devorch 소스트리와 현재 1단계 구현 상태를 비교했을 때, 지금 시점에서 빠진 것들입니다(설계상 다음 단계에서 채워야 하는 것들):

1) cmd 레벨

cmd/devorch-broker/main.go (Windows UAC / mac helper 브릿지) 미구현


2) internal/config (당신의 트리에는 있었던 것들)

schema.go, merge.go, jsonc.go, sources.go, validate.go 대부분 미구현(현재는 단순 json + env 몇 개만)

OpenCode 스타일의 우선순위 체인:
remote → global → custom env file → project → .devorch dirs → inline 미구현


3) internal/log

sink.go, rotate.go 미구현 (현재 stdout json만)


4) internal/storage

blob 분리 저장, sqlite migration 확장(usage 외) 미구현

하지만 1단계 최소코어 관점에서는 “정상”


5) internal/server

http2/grpc/sse/middleware/routes 다수 미구현 (현재 /health만)


👉 결론: 1단계 최소 구동 코어는 OK였고, 지금 바로 “멀티 모델/오케스트레이션”으로 가려면 설정 시스템이 먼저 완성되어야 합니다.
그래서 21단계는 config를 “진짜 OpenCode 스타일”로 올립니다.


---

B) 21단계: 설정 시스템(OpenCode 방식) 풀코드 추가

구현 범위 (이번 단계)

JSONC 파서(주석/후행 쉼표 처리: “현실적으로 안전한 수준”)

설정 소스/우선순위 체인

머지(깊은 merge) + validate(필수값/범위/정합)

로드 결과에 “어떤 파일/소스가 적용됐는지” 추적


> 주의: JSONC “완전 100%” 파서는 Go 표준만으로 만들기 어렵습니다.
여기서는 실무에서 충분히 먹히는 수준으로 // 및 /* */ 주석 제거 + trailing comma 제거를 제공합니다.
(나중에 필요하면 고급 파서로 교체해도 인터페이스는 유지됩니다.)




---

21-1) internal/config 구조 확장 파일들

internal/config/sources.go

package config

import (
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/global"
)

type SourceKind string

const (
	SourceDefaults SourceKind = "defaults"
	SourceRemote   SourceKind = "remote"   // placeholder
	SourceGlobal   SourceKind = "global"
	SourceCustom   SourceKind = "custom"   // env file path (DEVORCH_CONFIG)
	SourceProject  SourceKind = "project"  // repo root devorch.json(c)
	SourceDotDir   SourceKind = "dotdir"   // .devorch/devorch.json(c)
	SourceInline   SourceKind = "inline"   // DEVORCH_CONFIG_CONTENT
	SourceExtraDir SourceKind = "extradir" // DEVORCH_CONFIG_DIR
)

type Source struct {
	Kind SourceKind
	Path string // file path if applicable
	Note string // inline or remote note
}

type LoadPlan struct {
	Sources []Source
}

func BuildLoadPlan(paths global.Paths, env map[string]string, startDir string) LoadPlan {
	var out []Source
	out = append(out, Source{Kind: SourceDefaults})

	// (1) Remote config (.well-known/devorch) — placeholder (다음 단계에서 실제 fetch)
	// out = append(out, Source{Kind: SourceRemote, Note: ".well-known/devorch (not implemented)"})

	// (2) Global config: ~/.config/devorch/devorch.json(c)
	out = append(out, pickFirstExisting(
		Source{Kind: SourceGlobal, Path: filepath.Join(paths.Config, "devorch.jsonc")},
		Source{Kind: SourceGlobal, Path: filepath.Join(paths.Config, "devorch.json")},
	)...)

	// (3) Custom config file (env): DEVORCH_CONFIG
	if p := strings.TrimSpace(env["DEVORCH_CONFIG"]); p != "" {
		out = append(out, Source{Kind: SourceCustom, Path: p})
	}

	// (4) Project config (repo root): devorch.json(c)
	if root := findProjectRoot(startDir); root != "" {
		out = append(out, pickFirstExisting(
			Source{Kind: SourceProject, Path: filepath.Join(root, "devorch.jsonc")},
			Source{Kind: SourceProject, Path: filepath.Join(root, "devorch.json")},
		)...)

		// (5) .devorch/ override (최우선)
		out = append(out, pickFirstExisting(
			Source{Kind: SourceDotDir, Path: filepath.Join(root, ".devorch", "devorch.jsonc")},
			Source{Kind: SourceDotDir, Path: filepath.Join(root, ".devorch", "devorch.json")},
		)...)

		// 추가 조각 설정들(선택) - 앞으로 oauth/routing/quota 등으로 분해할 때 대비
		// 이 단계에서는 plan만 잡고 실제 merge는 "devorch.json(c)"에만 적용.
	}

	// (6) Extra config dir: DEVORCH_CONFIG_DIR (여러 파일을 머지하는 확장 포인트)
	if dir := strings.TrimSpace(env["DEVORCH_CONFIG_DIR"]); dir != "" {
		// 디렉터리 아래 devorch*.json(c)를 모두 읽어서 적용 (파일명 정렬)
		out = append(out, Source{Kind: SourceExtraDir, Path: dir})
	}

	// (7) Inline content: DEVORCH_CONFIG_CONTENT (최종 override)
	if s := strings.TrimSpace(env["DEVORCH_CONFIG_CONTENT"]); s != "" {
		out = append(out, Source{Kind: SourceInline, Note: "DEVORCH_CONFIG_CONTENT"})
	}

	return LoadPlan{Sources: out}
}

func pickFirstExisting(candidates ...Source) []Source {
	for _, c := range candidates {
		if c.Path == "" {
			continue
		}
		if _, err := os.Stat(c.Path); err == nil {
			return []Source{c}
		}
	}
	return nil
}

// findProjectRoot: .git 또는 go.mod 기준으로 상위 탐색
func findProjectRoot(start string) string {
	if start == "" {
		start, _ = os.Getwd()
	}
	dir := start
	for i := 0; i < 50; i++ {
		if dir == "" || dir == "/" || dir == "." {
			return ""
		}
		if exists(filepath.Join(dir, ".git")) || exists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}


---

internal/config/jsonc.go

package config

import (
	"bytes"
	"unicode/utf8"
)

// StripJSONC: //, /* */ 주석 제거 + trailing comma 제거(간단 처리)
func StripJSONC(in []byte) []byte {
	// 1) 주석 제거(문자열 내부는 보호)
	noComments := stripComments(in)
	// 2) trailing comma 제거(문자열 내부는 보호)
	noTrailing := stripTrailingCommas(noComments)
	return bytes.TrimSpace(noTrailing)
}

func stripComments(in []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(in))

	inStr := false
	escape := false

	for i := 0; i < len(in); i++ {
		c := in[i]

		if inStr {
			out.WriteByte(c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}

		if c == '"' {
			inStr = true
			out.WriteByte(c)
			continue
		}

		// line comment //
		if c == '/' && i+1 < len(in) && in[i+1] == '/' {
			// skip until newline
			i += 2
			for i < len(in) && in[i] != '\n' {
				i++
			}
			if i < len(in) {
				out.WriteByte('\n')
			}
			continue
		}

		// block comment /* */
		if c == '/' && i+1 < len(in) && in[i+1] == '*' {
			i += 2
			for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
				i++
			}
			i++ // skip '/'
			continue
		}

		out.WriteByte(c)
	}
	return out.Bytes()
}

func stripTrailingCommas(in []byte) []byte {
	// 매우 단순하지만 실무에서 잘 먹히는 방식:
	// 문자열 밖에서 , 다음에 ] 또는 } 가 나오면 , 제거
	var out bytes.Buffer
	out.Grow(len(in))

	inStr := false
	escape := false

	for i := 0; i < len(in); i++ {
		c := in[i]

		if inStr {
			out.WriteByte(c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}

		if c == '"' {
			inStr = true
			out.WriteByte(c)
			continue
		}

		if c == ',' {
			// lookahead to next non-space
			j := i + 1
			for j < len(in) {
				r, size := utf8.DecodeRune(in[j:])
				if r == utf8.RuneError && size == 1 {
					break
				}
				if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
					j += size
					continue
				}
				break
			}
			if j < len(in) && (in[j] == ']' || in[j] == '}') {
				// skip this comma
				continue
			}
		}

		out.WriteByte(c)
	}
	return out.Bytes()
}


---

internal/config/schema.go

package config

import "devorch/internal/global"

// SchemaVersion: 향후 config schema migration용
const SchemaVersion = 1

// Defaults: 1단계의 defaults.go를 유지하되 확장 포인트 명확화
func Defaults(paths global.Paths) Config {
	return Config{
		Server: ServerConfig{
			Host:              global.DefaultHTTPHost,
			Port:              global.DefaultHTTPPort,
			ShutdownTimeoutMs: 5000,
		},
		Storage: StorageConfig{
			Driver: "sqlite",
			SQLite: SQLiteConfig{
				Path: paths.Data + "/devorch.sqlite",
			},
		},
	}
}

> 기존 internal/config/defaults.go는 위로 대체하세요(파일명은 defaults.go로 두셔도 되고 schema.go로 합쳐도 됩니다).




---

internal/config/merge.go

package config

// Deep merge (현재 Config 구조 기준)
// - b의 non-zero 값이 a를 덮음
// - 앞으로 구조가 커지면 여기에 계속 추가
func Merge(a, b Config) Config {
	// server
	if b.Server.Host != "" {
		a.Server.Host = b.Server.Host
	}
	if b.Server.Port != 0 {
		a.Server.Port = b.Server.Port
	}
	if b.Server.ShutdownTimeoutMs != 0 {
		a.Server.ShutdownTimeoutMs = b.Server.ShutdownTimeoutMs
	}

	// storage
	if b.Storage.Driver != "" {
		a.Storage.Driver = b.Storage.Driver
	}
	if b.Storage.SQLite.Path != "" {
		a.Storage.SQLite.Path = b.Storage.SQLite.Path
	}

	return a
}


---

internal/config/validate.go

package config

import (
	"errors"
	"fmt"
	"strings"
)

func Validate(cfg Config) error {
	// server
	if strings.TrimSpace(cfg.Server.Host) == "" {
		return errors.New("server.host required")
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port out of range: %d", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeoutMs < 0 {
		return errors.New("server.shutdown_timeout_ms must be >= 0")
	}

	// storage
	if cfg.Storage.Driver == "" {
		return errors.New("storage.driver required")
	}
	switch cfg.Storage.Driver {
	case "sqlite":
		if strings.TrimSpace(cfg.Storage.SQLite.Path) == "" {
			return errors.New("storage.sqlite.path required for sqlite driver")
		}
	default:
		return fmt.Errorf("unsupported storage.driver: %s", cfg.Storage.Driver)
	}

	return nil
}


---

internal/config/load.go (교체)

package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"devorch/internal/global"
	"devorch/internal/log"
)

type LoadInput struct {
	Paths  global.Paths
	Env    []string
	Logger *log.Logger
}

type AppliedSource struct {
	Kind SourceKind
	Path string
	Note string
}

func Load(in LoadInput) (Config, string, error) {
	env := envToMap(in.Env)

	// 기본값
	cfg := Defaults(in.Paths)
	applied := []AppliedSource{{Kind: SourceDefaults}}

	startDir, _ := os.Getwd()
	plan := BuildLoadPlan(in.Paths, env, startDir)

	for _, src := range plan.Sources {
		switch src.Kind {
		case SourceDefaults:
			// already applied
			continue

		case SourceInline:
			// inline json/jsonc
			raw := []byte(env["DEVORCH_CONFIG_CONTENT"])
			parsed, err := parseConfigBytes(raw)
			if err != nil {
				return Config{}, "", err
			}
			cfg = Merge(cfg, parsed)
			applied = append(applied, AppliedSource{Kind: src.Kind, Note: src.Note})

		case SourceExtraDir:
			// directory 아래 devorch*.json(c) 전부 머지
			files, err := readConfigDir(src.Path)
			if err != nil {
				return Config{}, "", err
			}
			for _, f := range files {
				parsed, err := parseConfigFile(f)
				if err != nil {
					return Config{}, "", err
				}
				cfg = Merge(cfg, parsed)
				applied = append(applied, AppliedSource{Kind: SourceExtraDir, Path: f})
			}

		default:
			if src.Path == "" {
				continue
			}
			parsed, err := parseConfigFile(src.Path)
			if err != nil {
				// 존재하지 않으면 스킵(단, DEVORCH_CONFIG로 지정된 경우는 에러)
				if src.Kind == SourceCustom && errors.Is(err, os.ErrNotExist) {
					return Config{}, "", err
				}
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return Config{}, "", err
			}
			cfg = Merge(cfg, parsed)
			applied = append(applied, AppliedSource{Kind: src.Kind, Path: src.Path, Note: src.Note})
		}
	}

	if err := Validate(cfg); err != nil {
		return Config{}, "", err
	}

	return cfg, renderApplied(applied), nil
}

func parseConfigFile(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return parseConfigBytes(b)
}

func parseConfigBytes(b []byte) (Config, error) {
	clean := StripJSONC(b)
	var out Config
	if err := json.Unmarshal(clean, &out); err != nil {
		return Config{}, err
	}
	return out, nil
}

func readConfigDir(dir string) ([]string, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, errors.New("DEVORCH_CONFIG_DIR is not a directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if !strings.HasPrefix(low, "devorch") {
			continue
		}
		if strings.HasSuffix(low, ".json") || strings.HasSuffix(low, ".jsonc") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

func renderApplied(a []AppliedSource) string {
	parts := make([]string, 0, len(a))
	for _, s := range a {
		switch {
		case s.Path != "":
			parts = append(parts, string(s.Kind)+":"+s.Path)
		case s.Note != "":
			parts = append(parts, string(s.Kind)+":"+s.Note)
		default:
			parts = append(parts, string(s.Kind))
		}
	}
	return strings.Join(parts, " -> ")
}

// envToMap 동일(기존 load.go에서 쓰던거 그대로)
func envToMap(env []string) map[string]string {
	m := map[string]string{}
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				m[e[:i]] = e[i+1:]
				break
			}
		}
	}
	return m
}


---

21-2) 설정 파일 예시(이제 실제로 동작함)

~/.config/devorch/devorch.jsonc

{
  "server": {
    "host": "127.0.0.1",
    "port": 4096
  },
  "storage": {
    "driver": "sqlite",
    "sqlite": {
      "path": "/Users/you/.local/share/devorch/devorch.sqlite"
    }
  }
}

프로젝트 override: <repo>/.devorch/devorch.jsonc

{
  // 프로젝트별로만 포트 바꾸기
  "server": { "port": 4199 }
}


---

C) 반영 시 “수정해야 하는 기존 파일” (최소)

1) internal/config/defaults.go 는 삭제하거나, 위 schema.go로 대체

현재 1단계에 있던 Defaults() 함수는 그대로 쓰되, “21단계 버전”으로 교체하세요.


2) cmd/devorchd/main.go 는 그대로 두셔도 됩니다

config.Load() 시그니처가 동일해서 수정 필요 없음

로그에 config loaded source: defaults -> ... 같이 적용 경로가 찍힙니다.



---

D) 다음(22단계) 무엇을 이어서 개발할까?

이제 설정이 안정화되었으니, 다음은 딱 이 순서가 “개발 효율/리스크”가 제일 좋습니다.

1. security/secrets + OS별 token store 추상화 틀 (아직 OAuth 전에 기반 필요)


2. auth/OAuth PKCE + device flow (providers: GitHub/Microsoft/Google/OpenAI/Anthropic/OIDC)


3. provider registry + streaming mux/demux + retry/circuitbreaker (로컬/원격 공통)




---

원하시는 대로 “계속 개발”로 진행하겠습니다.
다음 응답(22단계)은 security/secrets(키체인/credman/libsecret 인터페이스) + audit_trail 최소 구현을 풀코드로 이어갈까요, 아니면 곧바로 **OAuth(PKCE/device flow)**로 들어갈까요?