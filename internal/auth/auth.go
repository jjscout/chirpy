package auth

import (
	"fmt"
	"strings"
	"time"

	"net/http"

	"crypto/rand"

	"encoding/hex"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(expiresIn)
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Issuer:    "chirpy-access",
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Subject:   userID.String(),
		},
	)
	signedToken, err := token.SignedString([]byte(tokenSecret))
	return signedToken, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			return []byte(tokenSecret), nil
		},
	)
	if err != nil {
		return uuid.Nil, err
	}
	parsed, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, err
	}
	return parsed, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	bearer := headers.Get("Authorization")
	if bearer == "" {
		return "", fmt.Errorf("missing Authentication header")
	}
	if !strings.HasPrefix(bearer, "Bearer ") {
		return "", fmt.Errorf("malformed Authentication header")
	}
	bearer = strings.TrimPrefix(bearer, "Bearer ")
	return bearer, nil
}

func MakeRefreshToken() string {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)
}
