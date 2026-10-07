package onemax

import (
	"regexp"
	"testing"
)

var base64URL = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func TestChallengeForMatchesRFC7636(t *testing.T) {
	got := ChallengeFor("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Errorf("ChallengeFor = %q", got)
	}
}

func TestGeneratePKCE(t *testing.T) {
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if !base64URL.MatchString(pkce.Verifier) {
		t.Errorf("Verifier = %q, want 43 base64url characters", pkce.Verifier)
	}
	if !base64URL.MatchString(pkce.Challenge) {
		t.Errorf("Challenge = %q, want 43 base64url characters", pkce.Challenge)
	}
	if pkce.Challenge != ChallengeFor(pkce.Verifier) {
		t.Errorf("Challenge does not match the verifier")
	}
}

func TestGeneratePKCEDiffersBetweenCalls(t *testing.T) {
	first, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	second, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if first.Verifier == second.Verifier || first.Challenge == second.Challenge {
		t.Errorf("two calls returned the same pair")
	}
}
