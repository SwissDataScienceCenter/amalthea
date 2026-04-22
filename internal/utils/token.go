package utils

import (
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// IsNotExpired returns true if expiresAt is still in the future, with a given margin.
// When expiresAt is zero, returns !required.
func IsNotExpired(expiresAt time.Time, margin time.Duration, required bool) bool {
	now := time.Now()
	deadline := now.Add(-margin)

	result := false
	if expiresAt.IsZero() {
		result = !required
	} else {
		result = expiresAt.After(deadline)
	}

	slog.Info("IsNotExpired", "result", result, "expiresAt", expiresAt.String(), "now", now.String(), "margin", margin.String(), "deadline", deadline.String(), "required", required)

	return result
}

// VerifyExpiresAtClaim returns true is the expiry (claims.GetExpirationTime()) is still in the future, with a given margin.
// When the expiry is omitted, returns !required.
func VerifyExpiresAtClaim(claims jwt.RegisteredClaims, margin time.Duration, required bool) (bool, error) {
	expiresAt, err := claims.GetExpirationTime()
	if err != nil {
		return false, err
	}
	if expiresAt == nil {
		return !required, nil
	}
	return IsNotExpired(expiresAt.Time, margin, required), nil
}

// VerifyJWTExpiresAt returns true is the expiry (from the "exp" claim) is still in the future, with a given margin.
// When the expiry is omitted, returns !required.
func VerifyJWTExpiresAt(token string, margin time.Duration, required bool) (bool, error) {
	parser := jwt.NewParser()
	claims := jwt.RegisteredClaims{}
	if _, _, err := parser.ParseUnverified(token, &claims); err != nil {
		slog.Error("Cannot parse token claims, assuming token is expired", "error", err)
		return false, err
	}
	return VerifyExpiresAtClaim(claims, margin, required)
}
