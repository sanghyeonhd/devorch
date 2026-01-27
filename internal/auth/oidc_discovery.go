package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type OIDCDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	DeviceEndpoint        string `json:"device_authorization_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

func DiscoverOIDC(ctx context.Context, issuer string) (OIDCDiscovery, error) {
	if issuer == "" {
		return OIDCDiscovery{}, errors.New("issuer empty")
	}
	issuer = strings.TrimRight(issuer, "/")
	u := issuer + "/.well-known/openid-configuration"

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	cl := &http.Client{Timeout: 15 * time.Second}
	res, err := cl.Do(req)
	if err != nil {
		return OIDCDiscovery{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return OIDCDiscovery{}, errors.New("oidc discovery http " + res.Status)
	}
	var d OIDCDiscovery
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		return OIDCDiscovery{}, err
	}
	return d, nil
}
