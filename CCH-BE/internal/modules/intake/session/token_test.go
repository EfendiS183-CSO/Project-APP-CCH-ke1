package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/config"
)

const testSigningKey = "test-only-intake-session-signing-key"

func TestServiceIssuesOpaqueGuestConversationToken(t *testing.T) {
	service, err := NewService(testSigningKey, "cch-intake-test", 20*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	issued, err := service.Issue()
	if err != nil {
		t.Fatal(err)
	}
	if len(issued.ConversationID) != 32 || issued.Token == "" || issued.ExpiresAt.Before(time.Now()) {
		t.Fatalf("unexpected issued conversation: %#v", issued)
	}
	verified, err := service.Verify(issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ConversationID != issued.ConversationID || !verified.ExpiresAt.Equal(issued.ExpiresAt) {
		t.Fatalf("verified conversation = %#v, issued = %#v", verified, issued)
	}
	if containsAny(issued.Token, "agent.test", "40005CC", "customer@example.test") {
		t.Fatal("guest conversation token contains personal or CCH staff data")
	}
}

func TestServiceRejectsInvalidConfiguration(t *testing.T) {
	for _, test := range []struct {
		name      string
		key       string
		issuer    string
		ttl       time.Duration
		wantError error
	}{
		{name: "short key", key: "short", issuer: "issuer", ttl: time.Minute, wantError: ErrInvalidSigningKey},
		{name: "empty issuer", key: testSigningKey, issuer: "", ttl: time.Minute, wantError: ErrInvalidIssuer},
		{name: "zero TTL", key: testSigningKey, issuer: "issuer", ttl: 0, wantError: ErrInvalidTTL},
		{name: "excessive TTL", key: testSigningKey, issuer: "issuer", ttl: maxConversationTTL + time.Second, wantError: ErrInvalidTTL},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewService(test.key, test.issuer, test.ttl); !errors.Is(err, test.wantError) {
				t.Fatalf("NewService() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestServiceRejectsTamperedExpiredAndWrongPurposeTokens(t *testing.T) {
	service, err := NewService(testSigningKey, "cch-intake-test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Issue()
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{
		issued.Token + "tampered",
		signedTestToken(t, claims{
			Purpose:        conversationPurpose,
			ConversationID: "expired-conversation",
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "cch-intake-test",
				Subject:   "expired-conversation",
				Audience:  jwt.ClaimStrings{conversationAudience},
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-30 * time.Minute)),
			},
		}),
		signedTestToken(t, claims{
			Purpose:        "cch-access-token",
			ConversationID: "wrong-purpose",
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "cch-intake-test",
				Subject:   "wrong-purpose",
				Audience:  jwt.ClaimStrings{conversationAudience},
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
		}),
	} {
		if _, err := service.Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Verify() error = %v, want invalid conversation token", err)
		}
	}
}

func TestIntakeConversationTokenCannotAuthenticateAsLegacyCCHJWT(t *testing.T) {
	service, err := NewService(testSigningKey, "cch-intake-test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Issue()
	if err != nil {
		t.Fatal(err)
	}
	cchVerifier, err := authentication.NewJWTVerifier(config.JWTSettings{Secret: testSigningKey, Issuer: "cch-intake-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cchVerifier.Verify(issued.Token); !errors.Is(err, authentication.ErrInvalidToken) {
		t.Fatalf("CCH JWT verifier accepted intake token: %v", err)
	}
}

func signedTestToken(t *testing.T, tokenClaims claims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims).SignedString([]byte(testSigningKey))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return token
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
