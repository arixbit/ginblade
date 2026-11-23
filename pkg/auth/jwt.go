package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds JWT signing and validation settings.
type JWTConfig struct {
	Secret string
	Issuer string
	TTL    time.Duration
}

// JWTManager signs and parses HS256 JWT tokens.
type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

// Claims is the generic JWT payload used by the skeleton.
type Claims struct {
	jwt.RegisteredClaims
}

var (
	// ErrMissingSecret is returned when JWTConfig.Secret is empty.
	ErrMissingSecret = errors.New("jwt secret is required")
	// ErrMissingSubject is returned when a token subject is empty.
	ErrMissingSubject = errors.New("jwt subject is required")
)

const defaultJWTTTL = 24 * time.Hour

// NewJWTManager creates a JWTManager from config.
func NewJWTManager(cfg JWTConfig) (*JWTManager, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, ErrMissingSecret
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultJWTTTL
	}
	return &JWTManager{
		secret: []byte(cfg.Secret),
		issuer: strings.TrimSpace(cfg.Issuer),
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

// GenerateToken signs a token for subject.
func (m *JWTManager) GenerateToken(subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", ErrMissingSubject
	}

	now := m.now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	if m.ttl > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(m.ttl))
	}

	return m.GenerateTokenWithClaims(claims)
}

// GenerateTokenWithClaims signs caller-provided claims.
func (m *JWTManager) GenerateTokenWithClaims(claims Claims) (string, error) {
	if strings.TrimSpace(claims.Subject) == "" {
		return "", ErrMissingSubject
	}
	if claims.Issuer == "" {
		claims.Issuer = m.issuer
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt token: %w", err)
	}
	return signed, nil
}

// ParseToken validates a Bearer or raw HS256 JWT token.
func (m *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	tokenString = normalizeBearerToken(tokenString)
	if tokenString == "" {
		return nil, errors.New("jwt token is required")
	}

	options := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time {
			return m.now().UTC()
		}),
	}
	if m.issuer != "" {
		options = append(options, jwt.WithIssuer(m.issuer))
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, options...)
	if err != nil {
		return nil, fmt.Errorf("parse jwt token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid jwt token")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, ErrMissingSubject
	}
	return claims, nil
}

func normalizeBearerToken(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return strings.TrimSpace(token[len("bearer "):])
	}
	return token
}
