package local

type Inventory struct {
	Runtime    string
	Version    string
	BinaryPath string
	Host       string

	HW HWInfo
}
