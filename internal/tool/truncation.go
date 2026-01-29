package tool

import (
	"strings"
	"unicode/utf8"
)

// Truncation limits
const (
	MaxBytes      = 50 * 1024 // 50KB max output
	MaxLines      = 2000      // Max lines for file reading
	MaxLineLength = 2000      // Max characters per line
	MetadataMax   = 30000     // Max metadata preview length
)

// TruncateOutput truncates output to fit within limits
func TruncateOutput(output string, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		maxBytes = MaxBytes
	}

	if len(output) <= maxBytes {
		return output, false
	}

	// Truncate to maxBytes, but ensure we don't cut in the middle of a UTF-8 character
	truncated := output[:maxBytes]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}

	return truncated + "\n\n... (output truncated)", true
}

// TruncateLines truncates to max lines
func TruncateLines(lines []string, maxLines int) ([]string, bool) {
	if maxLines <= 0 {
		maxLines = MaxLines
	}

	if len(lines) <= maxLines {
		return lines, false
	}

	return lines[:maxLines], true
}

// TruncateLine truncates a single line to max length
func TruncateLine(line string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = MaxLineLength
	}

	if len(line) <= maxLen {
		return line
	}

	return line[:maxLen] + "..."
}

// FormatFileContent formats file content with line numbers
func FormatFileContent(content string, offset int) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder

	sb.WriteString("<file>\n")
	for i, line := range lines {
		lineNum := offset + i + 1
		sb.WriteString(padLineNumber(lineNum, 5))
		sb.WriteString("| ")
		sb.WriteString(TruncateLine(line, MaxLineLength))
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n</file>")

	return sb.String()
}

// padLineNumber pads line number with zeros
func padLineNumber(n, width int) string {
	s := make([]byte, width)
	for i := range s {
		s[i] = '0'
	}
	str := []byte(itoa(n))
	copy(s[width-len(str):], str)
	return string(s)
}

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
