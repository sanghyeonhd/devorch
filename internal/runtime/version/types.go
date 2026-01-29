package version

type RuntimeInfo struct {
	CPUModel string
	CPUCores int

	GPUModel   string
	GPUDriver  string
	GPUBackend string

	RAMGB float64

	OllamaVersion   string
	LlamaCppVersion string
}
