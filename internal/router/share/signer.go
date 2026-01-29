package share

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
)

// Signer: export envelope 서명/검증
type Signer struct {
	Secret []byte
}

func NewSigner(secret []byte) *Signer {
	return &Signer{Secret: secret}
}

type SignedEnvelope struct {
	Payload   string `json:"payload"`   // base64 encoded JSON
	Signature string `json:"signature"` // HMAC-SHA256
}

func (s *Signer) Sign(env ExportEnvelope) (SignedEnvelope, error) {
	data, err := json.Marshal(env)
	if err != nil {
		return SignedEnvelope{}, err
	}

	payload := base64.StdEncoding.EncodeToString(data)
	sig := s.computeSig(data)

	return SignedEnvelope{
		Payload:   payload,
		Signature: sig,
	}, nil
}

func (s *Signer) Verify(se SignedEnvelope) (ExportEnvelope, error) {
	data, err := base64.StdEncoding.DecodeString(se.Payload)
	if err != nil {
		return ExportEnvelope{}, err
	}

	expected := s.computeSig(data)
	if !hmac.Equal([]byte(se.Signature), []byte(expected)) {
		return ExportEnvelope{}, errors.New("share: signature mismatch")
	}

	var env ExportEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return ExportEnvelope{}, err
	}
	return env, nil
}

func (s *Signer) computeSig(data []byte) string {
	h := hmac.New(sha256.New, s.Secret)
	h.Write(data)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
