package diagnostics

import (
	"context"
	"fmt"
	"time"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/log"
)

func RunDoctor(ctx context.Context, cfg config.Config, deps *app.Deps) error {
	log.Infof("doctor: db=%s", cfg.DBPath)

	// provider health checks
	for _, pinfo := range deps.ProviderRegistry.List() {
		p, _ := deps.ProviderRegistry.Get(pinfo.Name)
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := p.Health(cctx)
		cancel()
		if err != nil {
			return fmt.Errorf("provider %s health failed: %w", pinfo.Name, err)
		}
		log.Infof("doctor: provider %s health ok", pinfo.Name)
	}
	return nil
}
