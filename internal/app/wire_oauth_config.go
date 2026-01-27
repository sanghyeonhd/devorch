package app

import (
	"context"

	"devorch/internal/auth"
	"devorch/internal/config"
)

func OAuthConfigToProviderConfigs(ctx context.Context, oc config.OAuthConfig) map[auth.Provider]auth.ProviderConfig {
	result := make(map[auth.Provider]auth.ProviderConfig, len(oc.Providers))
	for name, cfg := range oc.Providers {
		pc, _ := auth.BuildProviderConfig(ctx, cfg)
		result[auth.Provider(name)] = pc
	}
	return result
}
