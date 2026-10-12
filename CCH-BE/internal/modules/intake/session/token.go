package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	minimumSigningKeyLength = 32
	conversationAudience    = "cch-intake"
	conversationPurpose     = "cch_intake_session"
	maxConversationTTL      = 24 * time.Hour
)

var (
	ErrServiceNotConfigured = errors.New("intake conversation token service is not configured")
	ErrInvalidSigningKey    = errors.New("intake conversation signing key is invalid")
	ErrInvalidIssuer        = errors.New("intake conversation issuer is invalid")
	ErrInvalidTTL           = errors.New("intake conversation token lifetime is invalid")
	ErrInvalidToken         = errors.New("invalid intake conversation token")
)

type claims struct {
	Purpose        string `json:"purpose"`
	ConversationID string `json:"conversation_id"`
	jwt.RegisteredClaims
}

type Service struct {
	signingKey []byte
	issuer     string
	ttl        time.Duration
}

type IssuedConversation struct {
	ConversationID string
	Token          string
	ExpiresAt      time.Time
}

type VerifiedConversation struct {
	ConversationID string
	ExpiresAt      time.Time
}

func NewService(signingKey string, issuer string, ttl time.Duration) (*Service, error) {
	if len([]byte(signingKey)) < minimumSigningKeyLength {
		return nil, ErrInvalidSigningKey
	}
	if issuer == "" {
		return nil, ErrInvalidIssuer
	}
	if ttl <= 0 || ttl > maxConversationTTL {
		return nil, ErrInvalidTTL
	}
	return &Service{signingKey: []byte(signingKey), issuer: issuer, ttl: ttl}, nil
}

func (service *Service) Issue() (IssuedConversation, error) {
	if service == nil || len(service.signingKey) < minimumSigningKeyLength || service.issuer == "" || service.ttl <= 0 {
		return IssuedConversation{}, ErrServiceNotConfigured
	}
	conversationID, err := newConversationID()
	if err != nil {
		return IssuedConversation{}, fmt.Errorf("create intake conversation ID: %w", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(service.ttl)
	tokenClaims := claims{
		Purpose:        conversationPurpose,
		ConversationID: conversationID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    service.issuer,
			Subject:   conversationID,
			Audience:  jwt.ClaimStrings{conversationAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims).SignedString(service.signingKey)
	if err != nil {
		return IssuedConversation{}, fmt.Errorf("sign intake conversation token: %w", err)
	}
	return IssuedConversation{
		ConversationID: conversationID,
		Token:          token,
		ExpiresAt:      expiresAt,
	}, nil
}

func (service *Service) Verify(tokenString string) (VerifiedConversation, error) {
	if service == nil || len(service.signingKey) < minimumSigningKeyLength || service.issuer == "" || service.ttl <= 0 {
		return VerifiedConversation{}, ErrServiceNotConfigured
	}
	tokenClaims := &claims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		tokenClaims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrInvalidToken
			}
			return service.signingKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(service.issuer),
		jwt.WithAudience(conversationAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || token == nil || !token.Valid || tokenClaims.Purpose != conversationPurpose ||
		tokenClaims.ConversationID == "" || tokenClaims.Subject != tokenClaims.ConversationID || tokenClaims.ExpiresAt == nil {
		return VerifiedConversation{}, ErrInvalidToken
	}
	return VerifiedConversation{
		ConversationID: tokenClaims.ConversationID,
		ExpiresAt:      tokenClaims.ExpiresAt.Time,
	}, nil
}

func newConversationID() (string, error) {
	var identifier [16]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(identifier[:]), nil
}
