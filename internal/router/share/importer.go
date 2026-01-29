package share

import (
	"context"
	"encoding/json"
	"errors"

	"devorch/internal/router"
)

// Importer: 외부 envelope을 로컬 policy store로 병합
type Importer struct {
	Store *router.PolicyStore
}

func NewImporter(store *router.PolicyStore) *Importer {
	return &Importer{Store: store}
}

func (i *Importer) Import(ctx context.Context, scopeType, scopeID string, data []byte) error {
	var env ExportEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	if env.Version == "" {
		return errors.New("share: invalid envelope version")
	}

	for _, e := range env.Policies {
		p := router.Policy{
			ScopeType:    scopeType,
			ScopeID:      scopeID,
			OS:           e.OS,
			Arch:         e.Arch,
			Scenario:     e.Scenario,
			Provider:     e.Provider,
			Model:        e.Model,
			Weight:       e.Weight,
			SuccessRate:  e.SuccessRate,
			AvgLatencyMs: e.AvgLatencyMs,
			AvgCost:      e.AvgCost,
			SampleCount:  e.SampleCount,
		}
		if err := i.Store.Upsert(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
