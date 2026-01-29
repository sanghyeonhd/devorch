package attribution

import (
	"context"
	"database/sql"

	"devorch/internal/id"
	"devorch/internal/runtime/version"
)

type EnvSigService struct {
	db *sql.DB
}

func NewEnvSigService(db *sql.DB) *EnvSigService {
	return &EnvSigService{db: db}
}

func (e *EnvSigService) CollectAndStore(ctx context.Context, os, arch, runtimeVer string) (EnvSignature, error) {
	rt := version.Detect()

	sig := EnvSignature{
		ID: id.NewULID(),

		OS:   os,
		Arch: arch,

		CPUModel: rt.CPUModel,
		CPUCores: rt.CPUCores,

		GPUModel:   rt.GPUModel,
		GPUDriver:  rt.GPUDriver,
		GPUBackend: rt.GPUBackend,

		RAMGB: rt.RAMGB,

		RuntimeVersion:  runtimeVer,
		OllamaVersion:   rt.OllamaVersion,
		LlamaCppVersion: rt.LlamaCppVersion,
	}

	_, err := e.db.ExecContext(ctx, `
INSERT INTO env_signatures
(id, os, arch, cpu_model, cpu_cores, gpu_model, gpu_driver, gpu_backend,
 ram_gb, runtime_version, ollama_version, llama_cpp_version)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		sig.ID, sig.OS, sig.Arch,
		sig.CPUModel, sig.CPUCores,
		sig.GPUModel, sig.GPUDriver, sig.GPUBackend,
		sig.RAMGB, sig.RuntimeVersion,
		sig.OllamaVersion, sig.LlamaCppVersion,
	)

	if err != nil {
		return EnvSignature{}, err
	}

	return sig, nil
}
