package authentication

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"cch.local/backend/internal/platform/config"
)

const minimumSigningKeyLength = 32

var ErrVerifierNotConfigured = errors.New("JWT verifier is not configured")
var ErrInvalidToken = errors.New("invalid bearer token")

type TokenVerifier interface {
	Verify(token string) (VerifiedIdentity, error)
}

type VerifiedIdentity struct {
	Username    string
	OfficeCode  string
	ReportTo    string
	UserLevelID string
	Permission  int64
}

type JWTVerifier struct {
	secret []byte
	issuer string
}

type cchClaims struct {
	Security cchSecurityClaims `json:"security"`
	jwt.RegisteredClaims
}

type cchSecurityClaims struct {
	Username     string `json:"username"`
	UserID       string `json:"userid"`
	ParentUserID string `json:"parentuserid"`
	UserLevelID  string `json:"userlevelid"`
	Permission   int64  `json:"permission"`
}

func NewJWTVerifier(settings config.JWTSettings) (*JWTVerifier, error) {
	if !settings.Configured() {
		return nil, ErrVerifierNotConfigured
	}
	if len([]byte(settings.Secret)) < minimumSigningKeyLength {
		return nil, fmt.Errorf("JWT signing key must be at least %d bytes", minimumSigningKeyLength)
	}

	return &JWTVerifier{
		secret: []byte(settings.Secret),
		issuer: settings.Issuer,
	}, nil
}

func (verifier *JWTVerifier) Verify(tokenString string) (VerifiedIdentity, error) {
	claims := &cchClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS512.Alg() {
				return nil, ErrInvalidToken
			}
			return verifier.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Alg()}),
		jwt.WithIssuer(verifier.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || token == nil || !token.Valid || claims.Security.Username == "" || claims.Security.UserLevelID == "" || claims.IssuedAt == nil {
		return VerifiedIdentity{}, ErrInvalidToken
	}

	return VerifiedIdentity{
		Username:    claims.Security.Username,
		OfficeCode:  claims.Security.UserID,
		ReportTo:    claims.Security.ParentUserID,
		UserLevelID: claims.Security.UserLevelID,
		Permission:  claims.Security.Permission,
	}, nil
}
