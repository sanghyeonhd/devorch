package streaming

import "io"

// EndOfStream: 스트림 종료를 표현하기 위해 io.EOF를 그대로 사용
var EndOfStream = io.EOF
