package onemax

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const verifierBytes = 32

type PKCE struct {
	Verifier  string
	Challenge string
}

func GeneratePKCE() (PKCE, error) {
	random := make([]byte, verifierBytes)
	if _, err := rand.Read(random); err != nil {
		return PKCE{}, fmt.Errorf("onemax: reading random bytes: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(random)
	return PKCE{Verifier: verifier, Challenge: ChallengeFor(verifier)}, nil
}

func ChallengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
