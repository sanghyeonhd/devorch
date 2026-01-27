package config

type OAuthProviderConfig struct {
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	AuthURL       string   `json:"auth_url,omitempty"`   // custom oidc
	TokenURL      string   `json:"token_url,omitempty"`  // custom oidc
	IssuerURL     string   `json:"issuer_url,omitempty"` // oidc discovery
	Scopes        []string `json:"scopes,omitempty"`
	DeviceAuthURL string   `json:"device_auth_url,omitempty"`
	Audience      string   `json:"audience,omitempty"`
}

type OAuthConfig struct {
	BaseURL   string                          `json:"base_url"` // callback base (daemon public)
	Providers map[string]OAuthProviderConfig `json:"providers"`
}
