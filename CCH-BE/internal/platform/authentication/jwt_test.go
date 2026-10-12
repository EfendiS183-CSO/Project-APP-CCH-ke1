package authentication

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"cch.local/backend/internal/platform/config"
)

const testSigningKey = "test-only-signing-key-with-at-least-32-bytes"

func TestNewJWTVerifierRequiresConfiguredStrongKey(t *testing.T) {
	if _, err := NewJWTVerifier(config.JWTSettings{}); err != ErrVerifierNotConfigured {
		t.Fatalf("error = %v, want verifier not configured", err)
	}
	if _, err := NewJWTVerifier(config.JWTSettings{Secret: "short", Issuer: "cch-test"}); err == nil {
		t.Fatal("expected short signing key to be rejected")
	}
}

func TestJWTVerifierMapsCCHIdentityClaims(t *testing.T) {
	verifier := newTestVerifier(t)
	token := signToken(t, jwt.SigningMethodHS512, testClaims(time.Now().Add(time.Minute)))

	identity, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if identity.Username != "agent.test" || identity.OfficeCode != "40005CC" || identity.ReportTo != "40005" || identity.UserLevelID != "3" || identity.Permission != 16 {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestJWTVerifierRejectsInvalidTokens(t *testing.T) {
	tests := []struct {
		name   string
		method *jwt.SigningMethodHMAC
		claims cchClaims
	}{
		{name: "wrong algorithm", method: jwt.SigningMethodHS256, claims: testClaims(time.Now().Add(time.Minute))},
		{name: "expired", method: jwt.SigningMethodHS512, claims: testClaims(time.Now().Add(-time.Minute))},
		{name: "missing expiration", method: jwt.SigningMethodHS512, claims: testClaims(time.Time{})},
		{
			name:   "missing identity",
			method: jwt.SigningMethodHS512,
			claims: cchClaims{RegisteredClaims: registeredClaims(time.Now().Add(time.Minute))},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier := newTestVerifier(t)
			token := signToken(t, test.method, test.claims)
			if _, err := verifier.Verify(token); err != ErrInvalidToken {
				t.Fatalf("error = %v, want invalid token", err)
			}
		})
	}
}

func TestJWTVerifierRejectsWrongIssuer(t *testing.T) {
	verifier := newTestVerifier(t)
	claims := testClaims(time.Now().Add(time.Minute))
	claims.Issuer = "unexpected-issuer"
	token := signToken(t, jwt.SigningMethodHS512, claims)

	if _, err := verifier.Verify(token); err != ErrInvalidToken {
		t.Fatalf("error = %v, want invalid token", err)
	}
}

func newTestVerifier(t *testing.T) *JWTVerifier {
	t.Helper()
	verifier, err := NewJWTVerifier(config.JWTSettings{Secret: testSigningKey, Issuer: "cch-test"})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	return verifier
}

func signToken(t *testing.T, method jwt.SigningMethod, claims cchClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return token
}

func testClaims(expiration time.Time) cchClaims {
	return cchClaims{
		Security: cchSecurityClaims{
			Username:     "agent.test",
			UserID:       "40005CC",
			ParentUserID: "40005",
			UserLevelID:  "3",
			Permission:   16,
		},
		RegisteredClaims: registeredClaims(expiration),
	}
}

func registeredClaims(expiration time.Time) jwt.RegisteredClaims {
	claims := jwt.RegisteredClaims{
		Issuer:   "cch-test",
		IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}
	if !expiration.IsZero() {
		claims.ExpiresAt = jwt.NewNumericDate(expiration)
	}
	return claims
}
