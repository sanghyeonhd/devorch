package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func randURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}

func NewPKCE() (*PKCE, error) {
	v, err := randURLSafe(32)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(v))
	ch := base64.RawURLEncoding.EncodeToString(h[:])
	return &PKCE{Verifier: v, Challenge: ch, Method: "S256"}, nil
}
