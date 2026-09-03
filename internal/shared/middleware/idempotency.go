// Package middleware provides cross-cutting Gin middleware. This file holds
// the idempotency middleware + its GORM-backed store, owned here to avoid an
// import cycle with the modules that consume it.
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/response"
)

const (
	// IdempotencyKeyTTL is how long a stored response stays replayable.
	IdempotencyKeyTTL = 24 * time.Hour
	// IdempotencyHeader is the request header carrying the client key.
	IdempotencyHeader = "Idempotency-Key"
)

// IdempotencyKey is the GORM model for the idempotency_keys table.
type IdempotencyKey struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	KeyHash        string    `gorm:"type:text;not null;uniqueIndex"`
	UserID         uuid.UUID `gorm:"type:uuid;not null"`
	Endpoint       string    `gorm:"type:text;not null"`
	RequestHash    string    `gorm:"type:text;not null"`
	ResponseStatus int       `gorm:"not null"`
	ResponseBody   string    `gorm:"type:text;not null"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	ExpiresAt      time.Time `gorm:"not null"`
}

// TableName overrides the default table name.
func (IdempotencyKey) TableName() string { return "idempotency_keys" }

// IdempotencyStore owns the DB handle and TTL for replayable routes. It is
// built once in cmd/server and shared by every protected write route.
type IdempotencyStore struct {
	db  *gorm.DB
	ttl time.Duration
}

// NewIdempotencyStore builds a store with the default TTL.
func NewIdempotencyStore(db *gorm.DB) *IdempotencyStore {
	return &IdempotencyStore{db: db, ttl: IdempotencyKeyTTL}
}

// Middleware returns the idempotency handler. Requests without an
// Idempotency-Key header pass through untouched; with one, the first 2xx
// response is stored and replayed verbatim for identical retries, while a
// same-key/different-body retry is rejected with 409 DUPLICATE_REQUEST.
func (s *IdempotencyStore) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader(IdempotencyHeader)
		if key == "" {
			c.Next()
			return
		}

		userID := UserIDFromContext(c)
		route := c.FullPath()

		rawBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			response.Error(c, sharederr.ErrInternal)
			c.Abort()
			return
		}
		// Restore the stream so the handler still sees the body.
		c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))

		keyHash := hashKey(userID, route, key)
		reqHash := hashRequest(c.Request.Method, rawBody)

		stored, found, err := s.find(c.Request.Context(), keyHash)
		if err != nil {
			response.Error(c, sharederr.ErrInternal)
			c.Abort()
			return
		}
		if found {
			if stored.RequestHash == reqHash {
				c.Data(stored.ResponseStatus, "application/json; charset=utf-8", []byte(stored.ResponseBody))
				c.Abort()
				return
			}
			response.Error(c, sharederr.ErrDuplicateRequest)
			c.Abort()
			return
		}

		cap := &captureWriter{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = cap
		c.Next()

		if cap.code < http.StatusOK || cap.code >= http.StatusMultipleChoices {
			return // only 2xx responses are stored; failures stay retryable
		}
		_ = s.save(c.Request.Context(), IdempotencyKey{
			KeyHash:        keyHash,
			UserID:         userID,
			Endpoint:       route,
			RequestHash:    reqHash,
			ResponseStatus: cap.code,
			ResponseBody:   cap.buf.String(),
			ExpiresAt:      time.Now().Add(s.ttl),
		})
	}
}

func (s *IdempotencyStore) find(ctx context.Context, keyHash string) (IdempotencyKey, bool, error) {
	var row IdempotencyKey
	err := s.db.WithContext(ctx).Where("key_hash = ? AND expires_at > now()", keyHash).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IdempotencyKey{}, false, nil
	}
	if err != nil {
		return IdempotencyKey{}, false, err
	}
	return row, true, nil
}

// save inserts the stored response; a concurrent identical write is ignored
// (the loser's replay is served from the winner's row on the next retry).
func (s *IdempotencyStore) save(ctx context.Context, row IdempotencyKey) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key_hash"}}, DoNothing: true}).
		Create(&row).Error
}

// hashKey derives the storage key: user + route + client header, so identical
// keys from different users or routes never collide.
func hashKey(userID uuid.UUID, route, key string) string {
	h := sha256.New()
	h.Write([]byte(userID.String()))
	h.Write([]byte{0})
	h.Write([]byte(route))
	h.Write([]byte{0})
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// hashRequest derives the idempotency determinant from the verb + raw body.
func hashRequest(verb string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(hex.EncodeToString(sum([]byte(verb)))))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

// captureWriter buffers the response body + status so the middleware can
// store 2xx payloads verbatim while still streaming to the real writer.
type captureWriter struct {
	gin.ResponseWriter
	buf  *bytes.Buffer
	code int
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}