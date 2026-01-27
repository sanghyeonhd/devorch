package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

func LoadOAuthConfig(paths ...string) (OAuthConfig, error) {
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var c OAuthConfig
		if err := UnmarshalJSONC(b, &c); err != nil {
			return OAuthConfig{}, err
		}
		if c.Providers == nil {
			c.Providers = map[string]OAuthProviderConfig{}
		}
		return c, nil
	}
	return OAuthConfig{}, errors.New("oauth config not found")
}

func DefaultOAuthPaths(projectRoot string, userConfigDir string) []string {
	return []string{
		filepath.Join(projectRoot, ".devorch", "oauth.jsonc"),
		filepath.Join(userConfigDir, "devorch", "oauth.jsonc"),
	}
}

// UnmarshalJSONC strips comments from JSONC before unmarshaling
func UnmarshalJSONC(data []byte, v any) error {
	// Remove single-line comments
	re := regexp.MustCompile(`(?m)//.*$`)
	data = re.ReplaceAll(data, []byte{})
	// Remove multi-line comments
	re2 := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	data = re2.ReplaceAll(data, []byte{})
	return json.Unmarshal(data, v)
}
