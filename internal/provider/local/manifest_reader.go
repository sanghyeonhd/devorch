package local

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ModelRecommendation struct {
	ID       string `json:"id"`
	Family   string `json:"family"`
	MinCores int    `json:"min_cores"`
	MinMemMB int    `json:"min_mem_mb"`
	Notes    string `json:"notes"`
}

type Manifest struct {
	Version           string                `json:"version"`
	Runtimes          []string              `json:"runtimes"`
	RecommendedModels []ModelRecommendation `json:"recommended_models"`
}

func LoadManifest(modelsDir string) (Manifest, error) {
	p := filepath.Join(modelsDir, "manifest.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}
