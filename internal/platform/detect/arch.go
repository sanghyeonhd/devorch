package detect

import "runtime"

func Arch() string { return runtime.GOARCH }
