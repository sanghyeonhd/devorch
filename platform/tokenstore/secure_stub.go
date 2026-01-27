//go:build !darwin && !windows && !linux

package tokenstore

import (
	"context"
	"errors"

	"devorch/internal/auth"
)

type SecureStub struct{}

func (s *SecureStub) Get(ctx context.Context, ws string, p auth.Provider) (auth.Token, bool, error) {
	return auth.Token{}, false, errors.New("no secure store")
}
func (s *SecureStub) Put(ctx context.Context, ws string, p auth.Provider, tok auth.Token) error {
	return errors.New("no secure store")
}
func (s *SecureStub) Delete(ctx context.Context, ws string, p auth.Provider) error { return nil }
