package detect

import "runtime"

type Features struct {
	HasPTY      bool
	HasKeychain bool
	HasService  bool
}

func DetectFeatures() Features {
	switch runtime.GOOS {
	case "darwin":
		return Features{HasPTY: true, HasKeychain: true, HasService: true}
	case "windows":
		return Features{HasPTY: true, HasKeychain: true, HasService: true}
	default:
		return Features{HasPTY: true, HasKeychain: false, HasService: true}
	}
}
