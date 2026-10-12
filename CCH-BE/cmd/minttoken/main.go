package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Mint a dev HS512 CCH token for local testing using the same claims the
// verifier expects: nested "security" with userid/parentuserid/userlevelid.
func main() {
	secret := os.Getenv("CCH_JWT_SECRET")
	issuer := os.Getenv("CCH_JWT_ISSUER")
	if secret == "" || issuer == "" {
		fmt.Fprintln(os.Stderr, "CCH_JWT_SECRET and CCH_JWT_ISSUER are required")
		os.Exit(2)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": issuer,
		"iat": now.Unix(),
		"exp": now.Add(24 * time.Hour).Unix(),
		"security": map[string]any{
			"username":     "40005PI02",
			"userid":       "40005CC",
			"parentuserid": "40005",
			"userlevelid":  "3",
			"permission":   int64(0),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		fmt.Fprintln(os.Stderr, "sign: ", err)
		os.Exit(1)
	}
	fmt.Print(signed)
}
