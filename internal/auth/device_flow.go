package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type deviceTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Error        string `json:"error,omitempty"`
}

func (m *Manager) BeginDeviceFlow(ctx context.Context, p Provider) (DeviceCodeResponse, error) {
	pc, ok := m.Configs[p]
	if !ok {
		return DeviceCodeResponse{}, errors.New("provider not configured")
	}

	deviceURL := ""
	if pc.DeviceAuthURL != "" {
		deviceURL = pc.DeviceAuthURL
	} else if pc.AuthURL == "" && pc.TokenURL == "" && pc.IssuerURL != "" {
		d, err := DiscoverOIDC(ctx, pc.IssuerURL)
		if err != nil {
			return DeviceCodeResponse{}, err
		}
		deviceURL = d.DeviceEndpoint
		if deviceURL == "" {
			return DeviceCodeResponse{}, errors.New("issuer has no device endpoint")
		}
	} else {
		return DeviceCodeResponse{}, errors.New("device flow endpoints not configured")
	}

	form := url.Values{}
	form.Set("client_id", pc.ClientID)
	if len(pc.Scopes) > 0 {
		form.Set("scope", strings.Join(pc.Scopes, " "))
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", deviceURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	cl := &http.Client{Timeout: 20 * time.Second}
	res, err := cl.Do(req)
	if err != nil {
		return DeviceCodeResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return DeviceCodeResponse{}, errors.New("device auth http " + res.Status)
	}

	var out DeviceCodeResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return DeviceCodeResponse{}, err
	}
	if out.Interval == 0 {
		out.Interval = 5
	}
	return out, nil
}

func (m *Manager) PollDeviceFlow(ctx context.Context, p Provider, deviceCode string, intervalSec int) (Token, error) {
	pc, ok := m.Configs[p]
	if !ok {
		return Token{}, errors.New("provider not configured")
	}

	tokenURL := pc.TokenURL
	if tokenURL == "" && pc.IssuerURL != "" {
		d, err := DiscoverOIDC(ctx, pc.IssuerURL)
		if err != nil {
			return Token{}, err
		}
		tokenURL = d.TokenEndpoint
	}
	if tokenURL == "" {
		return Token{}, errors.New("token endpoint not configured")
	}

	interval := time.Duration(intervalSec) * time.Second
	if interval < 2*time.Second {
		interval = 2 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return Token{}, ctx.Err()
		default:
		}

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("client_id", pc.ClientID)
		form.Set("device_code", deviceCode)
		if pc.ClientSecret != "" {
			form.Set("client_secret", pc.ClientSecret)
		}

		req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		cl := &http.Client{Timeout: 20 * time.Second}
		res, err := cl.Do(req)
		if err != nil {
			return Token{}, err
		}

		var tr deviceTokenResponse
		_ = json.NewDecoder(res.Body).Decode(&tr)
		res.Body.Close()

		if tr.Error == "authorization_pending" {
			time.Sleep(interval)
			continue
		}
		if tr.Error == "slow_down" {
			interval += 2 * time.Second
			time.Sleep(interval)
			continue
		}
		if tr.Error != "" {
			return Token{}, errors.New("device flow error: " + tr.Error)
		}
		if tr.AccessToken == "" {
			return Token{}, errors.New("device flow: no access_token")
		}

		exp := time.Time{}
		if tr.ExpiresIn > 0 {
			exp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
		}
		return Token{
			AccessToken:  tr.AccessToken,
			RefreshToken: tr.RefreshToken,
			TokenType:    tr.TokenType,
			Expiry:       exp,
			Scope:        tr.Scope,
			IDToken:      tr.IDToken,
		}, nil
	}
}
