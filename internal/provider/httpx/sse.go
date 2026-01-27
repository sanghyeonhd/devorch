package httpx

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
)

type SSEEvent struct {
	Data string
}

func ReadSSE(ctx context.Context, r io.Reader, onEvent func(SSEEvent) error) error {
	sc := bufio.NewScanner(r)
	// SSE payload가 길면 기본 토큰 제한에 걸림 → 버퍼 확장
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 8*1024*1024)

	var data bytes.Buffer

	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		ev := SSEEvent{Data: strings.TrimSpace(data.String())}
		data.Reset()
		return onEvent(ev)
	}

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := sc.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			data.WriteString(payload)
			data.WriteString("\n")
		}
		// id:, event: 무시 (확장 가능)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return flush()
}
