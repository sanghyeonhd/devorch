package auth

import (
	"context"
	"time"

	"devorch/internal/config"
)

func BuildProviderConfig(ctx context.Context, oc config.OAuthProviderConfig) (ProviderConfig, error) {
	pc := ProviderConfig{
		ClientID:      oc.ClientID,
		ClientSecret:  oc.ClientSecret,
		AuthURL:       oc.AuthURL,
		TokenURL:      oc.TokenURL,
		IssuerURL:     oc.IssuerURL,
		DeviceAuthURL: oc.DeviceAuthURL,
		Scopes:        oc.Scopes,
		Audience:      oc.Audience,
	}

	// OIDC Discovery if IssuerURL exists and endpoints missing
	if pc.IssuerURL != "" && (pc.AuthURL == "" || pc.TokenURL == "") {
		discoverCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		d, err := DiscoverOIDC(discoverCtx, pc.IssuerURL)
		if err == nil {
			if pc.AuthURL == "" {
				pc.AuthURL = d.AuthorizationEndpoint
			}
			if pc.TokenURL == "" {
				pc.TokenURL = d.TokenEndpoint
			}
			if pc.DeviceAuthURL == "" && d.DeviceEndpoint != "" {
				pc.DeviceAuthURL = d.DeviceEndpoint
			}
		}
	}
	return pc, nil
}
