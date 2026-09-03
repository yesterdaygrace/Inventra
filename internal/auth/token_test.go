package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager() *TokenManager {
	return NewTokenManager(TokenManagerConfig{
		Secret:     "test-secret-that-is-long-enough-for-hs256",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	})
}

func TestSignAndParseAccessToken(t *testing.T) {
	tm := newTestManager()
	uid := uuid.New()
	raw, err := tm.SignAccessToken(uid, "ADMIN", nil)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	claims, err := tm.ParseAccessToken(raw)
	require.NoError(t, err)
	assert.Equal(t, uid.String(), claims.Subject)
	assert.Equal(t, "ADMIN", claims.Role)
	assert.Equal(t, tokenIssuer, claims.Issuer)
	assert.Contains(t, claims.Audience, tokenAudience)
}

func TestParseAccessTokenRejectsTampered(t *testing.T) {
	tm := newTestManager()
	uid := uuid.New()
	raw, err := tm.SignAccessToken(uid, "STAFF", nil)
	require.NoError(t, err)

	tampered := raw[:len(raw)-2] + "xx"
	_, err = tm.ParseAccessToken(tampered)
	assert.ErrorIs(t, err, ErrTokenInvalid)
}

func TestParseAccessTokenRejectsWrongSecret(t *testing.T) {
	uid := uuid.New()
	raw, err := newTestManager().SignAccessToken(uid, "STAFF", nil)
	require.NoError(t, err)

	other := NewTokenManager(TokenManagerConfig{
		Secret:     "a-completely-different-secret-value-1234567890",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	})
	_, err = other.ParseAccessToken(raw)
	assert.ErrorIs(t, err, ErrTokenInvalid)
}

func TestParseAccessTokenExpired(t *testing.T) {
	tm := NewTokenManager(TokenManagerConfig{
		Secret:     "test-secret-value-for-expired-token-case",
		AccessTTL:  -1 * time.Minute, // already expired
		RefreshTTL: 7 * 24 * time.Hour,
	})
	uid := uuid.New()
	raw, err := tm.SignAccessToken(uid, "STAFF", nil)
	require.NoError(t, err)

	_, err = tm.ParseAccessToken(raw)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestGenerateRefreshTokenUniqueAndHashed(t *testing.T) {
	tm := newTestManager()

	raw1, hash1, exp1, err := tm.GenerateRefreshToken()
	require.NoError(t, err)
	raw2, hash2, _, err := tm.GenerateRefreshToken()
	require.NoError(t, err)

	assert.NotEmpty(t, raw1)
	assert.Len(t, raw1, 56) // 40 bytes -> base64
	assert.NotEqual(t, raw1, raw2, "refresh tokens must be unique")
	assert.Equal(t, 64, len(hash1), "sha256 hex is 64 chars")

	// hash round-trips: HashRefreshToken(raw) == stored hash
	assert.Equal(t, hash1, tm.HashRefreshToken(raw1))
	assert.NotEqual(t, hash2, hash1)

	// expiry in the future
	assert.True(t, exp1.After(time.Now()))
	// raw must not contain the hash (no leakage)
	assert.False(t, strings.Contains(hash1, raw1))
}

func TestGenerateRefreshTokenRandomness(t *testing.T) {
	tm := newTestManager()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		raw, _, _, err := tm.GenerateRefreshToken()
		require.NoError(t, err)
		assert.False(t, seen[raw], "refresh token %s duplicated", raw)
		seen[raw] = true
	}
}

func TestRefreshTTL(t *testing.T) {
	tm := &TokenManager{secret: []byte("s"), accessTTL: time.Minute, refreshTTL: 48 * time.Hour}
	assert.Equal(t, 48*time.Hour, tm.RefreshTTL())
}
