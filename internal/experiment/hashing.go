package experiment

import (
	"crypto/sha256"
	"encoding/binary"
)

func hashToUint64(seed string, subject string) uint64 {
	sum := sha256.Sum256([]byte(seed + ":" + subject))
	return binary.LittleEndian.Uint64(sum[:8])
}

func hashToFloat01(seed string, subject string) float64 {
	v := hashToUint64(seed, subject)
	// 0..1
	return float64(v%1_000_000_000) / 1_000_000_000.0
}
