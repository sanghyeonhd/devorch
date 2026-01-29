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
