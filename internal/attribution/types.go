package attribution

type EnvSignature struct {
	ID string

	OS   string
	Arch string

	CPUModel string
	CPUCores int

	GPUModel   string
	GPUDriver  string
	GPUBackend string

	RAMGB float64

	RuntimeVersion  string
	OllamaVersion   string
	LlamaCppVersion string
}

type AttributionEvent struct {
	ID string

	BenchRunID     string
	EnvSignatureID string

	Model    string
	Provider string

	PromptHash    string
	PromptVersion string

	ToolchainHash string
	RuntimeFlags  string
}
