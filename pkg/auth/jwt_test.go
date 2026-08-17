package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTManagerGenerateAndParse(t *testing.T) {
	manager, err := NewJWTManager(JWTConfig{
		Secret: "test-secret",
		Issuer: "ginblade",
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}
	manager.now = func() time.Time {
		return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	}

	token, err := manager.GenerateToken("account-1")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := manager.ParseToken("Bearer " + token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Subject != "account-1" {
		t.Fatalf("Subject = %q, want account-1", claims.Subject)
	}
	if claims.Issuer != "ginblade" {
		t.Fatalf("Issuer = %q, want ginblade", claims.Issuer)
	}
}

func TestJWTManagerValidation(t *testing.T) {
	_, err := NewJWTManager(JWTConfig{})
	if !errors.Is(err, ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}

	manager, err := NewJWTManager(JWTConfig{Secret: "test-secret"})
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}
	if _, err := manager.GenerateToken(" "); !errors.Is(err, ErrMissingSubject) {
		t.Fatalf("expected ErrMissingSubject, got %v", err)
	}
}

func TestJWTManagerRejectsWrongIssuer(t *testing.T) {
	signer, err := NewJWTManager(JWTConfig{Secret: "test-secret", Issuer: "issuer-a"})
	if err != nil {
		t.Fatalf("NewJWTManager signer: %v", err)
	}
	verifier, err := NewJWTManager(JWTConfig{Secret: "test-secret", Issuer: "issuer-b"})
	if err != nil {
		t.Fatalf("NewJWTManager verifier: %v", err)
	}

	token, err := signer.GenerateToken("account-1")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := verifier.ParseToken(token); err == nil {
		t.Fatal("expected issuer validation error")
	}
}

func TestJWTManagerRejectsNonHS256(t *testing.T) {
	manager, err := NewJWTManager(JWTConfig{Secret: "test-secret"})
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "account-1"},
	})
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	if _, err := manager.ParseToken(signed); err == nil {
		t.Fatal("expected signing method validation error")
	}
}
