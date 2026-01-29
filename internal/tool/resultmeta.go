package tool

// Tool들이 "구조화된 결과 메타"를 선택적으로 제공하기 위한 인터페이스
// 예: exec tool => exit_code, changed_files, bytes_written
type ResultMeta interface {
	Meta() map[string]any
}
