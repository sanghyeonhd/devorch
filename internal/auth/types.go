package auth

import "time"

type Provider string

const (
	ProviderGitHub    Provider = "github"
	ProviderGoogle    Provider = "google"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderOIDC      Provider = "custom_oidc"
)

type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
	Scope        string
	IDToken      string
}

func (t Token) Expired(skew time.Duration) bool {
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().Add(skew).After(t.Expiry)
}
