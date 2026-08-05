// Token manager: HS256 JWT access tokens and rotating SHA-256-hashed
// refresh tokens.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessClaims are the claims carried in a signed access token.
type AccessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager signs and verifies access tokens and generates/hashes
// refresh tokens.
type TokenManager struct {
	secret      []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
	issuer      string
	audience    string
	refreshSize int
}

// TokenManagerConfig carries the configuration a TokenManager needs.
type TokenManagerConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

const (
	tokenIssuer   = "inventory-api"
	tokenAudience = "inventory"
	refreshBytes  = 40
)

var (
	// ErrTokenInvalid is returned when a token fails signature/format checks.
	ErrTokenInvalid = errors.New("invalid token")
	// ErrTokenExpired is returned when a token is valid but expired.
	ErrTokenExpired = errors.New("token expired")
)

// NewTokenManager builds a TokenManager from the given config.
func NewTokenManager(cfg TokenManagerConfig) *TokenManager {
	return &TokenManager{
		secret:      []byte(cfg.Secret),
		accessTTL:   cfg.AccessTTL,
		refreshTTL:  cfg.RefreshTTL,
		issuer:      tokenIssuer,
		audience:    tokenAudience,
		refreshSize: refreshBytes,
	}
}

// SignAccessToken issues a signed HS256 access token for the given user.
func (tm *TokenManager) SignAccessToken(userID uuid.UUID, role string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    tm.issuer,
			Audience:  jwt.ClaimStrings{tm.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(tm.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// ParseAccessToken validates an access token's signature, issuer,
// audience, and expiry, returning its claims.
func (tm *TokenManager) ParseAccessToken(raw string) (*AccessClaims, error) {
	var claims AccessClaims
	token, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return tm.secret, nil
	},
		jwt.WithIssuer(tm.issuer),
		jwt.WithAudience(tm.audience),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if !token.Valid {
		return nil, ErrTokenInvalid
	}
	claims = *token.Claims.(*AccessClaims)
	return &claims, nil
}

// GenerateRefreshToken returns a cryptographically random refresh token
// and its SHA-256 hex hash for storage, plus the expiry time.
func (tm *TokenManager) GenerateRefreshToken() (raw string, hash string, expiresAt time.Time, err error) {
	buf := make([]byte, tm.refreshSize)
	if _, err = rand.Read(buf); err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}
	raw = base64.StdEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	return raw, fmt.Sprintf("%x", sum[:]), time.Now().Add(tm.refreshTTL), nil
}

// HashRefreshToken returns the SHA-256 hex hash of a raw refresh token.
func (tm *TokenManager) HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:])
}

// RefreshTTL reports the configured refresh token lifetime.
func (tm *TokenManager) RefreshTTL() time.Duration { return tm.refreshTTL }
