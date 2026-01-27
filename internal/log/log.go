package log

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	std = log.New(os.Stdout, "", 0)
)

func Init() {
	std.SetFlags(0)
}

func ts() string {
	return time.Now().Format(time.RFC3339)
}

func Infof(format string, args ...any) {
	std.Printf("%s [INFO] %s\n", ts(), fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	std.Printf("%s [ERROR] %s\n", ts(), fmt.Sprintf(format, args...))
}
