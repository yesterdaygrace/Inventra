package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"inventory/internal/shared/middleware"
)

// setupIdempotencyDB provisions the idempotency_keys table on the shared
// :5433 Postgres, then cleans up. The users FK is dropped so tests can store
// an arbitrary user_id regardless of the current users-table shape.
func setupIdempotencyDB(t *testing.T) (*gorm.DB, uuid.UUID) {
	t.Helper()
	dsn := "host=localhost user=postgres password=postgres dbname=inventory port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS idempotency_keys CASCADE")
	})

	require.NoError(t, db.AutoMigrate(&middleware.IdempotencyKey{}))
	require.NoError(t, db.Exec("ALTER TABLE idempotency_keys DROP CONSTRAINT IF EXISTS idempotency_keys_user_id_fkey").Error)
	return db, uuid.New()
}

// buildIdemEngine returns a Gin engine with the idempotency middleware and a
// counting stub handler that echoes the request body as JSON.
func buildIdemEngine(t *testing.T, store *middleware.IdempotencyStore, userID uuid.UUID) (*gin.Engine, *atomic.Int32) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var calls atomic.Int32
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, userID)
		c.Next()
	})
	r.POST("/op", store.Middleware(), func(c *gin.Context) {
		calls.Add(1)
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read body"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"echo": string(body)})
	})
	return r, &calls
}

func doIdemReq(r *gin.Engine, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/op", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set(middleware.IdempotencyHeader, key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestIdempotencyMissingHeaderPassesThrough(t *testing.T) {
	db, userID := setupIdempotencyDB(t)
	store := middleware.NewIdempotencyStore(db)

	r, calls := buildIdemEngine(t, store, userID)

	w := doIdemReq(r, "", `{"qty":5}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(1), calls.Load(), "handler executes without a key")

	var count int64
	require.NoError(t, db.Model(&middleware.IdempotencyKey{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "no row stored without a key")
}

func TestIdempotencyReplaysIdenticalRequest(t *testing.T) {
	db, userID := setupIdempotencyDB(t)
	store := middleware.NewIdempotencyStore(db)

	r, calls := buildIdemEngine(t, store, userID)
	body := `{"qty":5}`

	first := doIdemReq(r, "key-abc", body)
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, int32(1), calls.Load())

	second := doIdemReq(r, "key-abc", body)
	assert.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, int32(1), calls.Load(), "replay must not execute the handler again")
	assert.Equal(t, first.Body.String(), second.Body.String(), "replay returns the stored body verbatim")
}

func TestIdempotencyDifferentBodySameKeyRejects(t *testing.T) {
	db, userID := setupIdempotencyDB(t)
	store := middleware.NewIdempotencyStore(db)

	r, calls := buildIdemEngine(t, store, userID)

	assert.Equal(t, http.StatusOK, doIdemReq(r, "key-abc", `{"qty":5}`).Code)

	dup := doIdemReq(r, "key-abc", `{"qty":99}`)
	assert.Equal(t, http.StatusConflict, dup.Code)
	assert.True(t, json.Valid(dup.Body.Bytes()))
	assert.Equal(t, int32(1), calls.Load(), "rejected retry never reaches the handler")
}

func TestIdempotencyDifferentKeysBothExecute(t *testing.T) {
	db, userID := setupIdempotencyDB(t)
	store := middleware.NewIdempotencyStore(db)

	r, calls := buildIdemEngine(t, store, userID)
	body := `{"qty":5}`

	assert.Equal(t, http.StatusOK, doIdemReq(r, "key-1", body).Code)
	assert.Equal(t, http.StatusOK, doIdemReq(r, "key-2", body).Code)
	assert.Equal(t, int32(2), calls.Load(), "different keys are independent movements")
}

func TestIdempotencyFailedAttemptNotStored(t *testing.T) {
	db, userID := setupIdempotencyDB(t)
	store := middleware.NewIdempotencyStore(db)

	r, _ := buildIdemEngine(t, store, userID)
	first := doIdemReq(r, "key-retry", `{"qty":5}`)
	require.Equal(t, http.StatusOK, first.Code)

	var count int64
	require.NoError(t, db.Model(&middleware.IdempotencyKey{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "success stored")

	// Simulate a failed attempt: a key whose handler returns 400 must not
	// persist, so a corrected retry is not blocked.
	gin.SetMode(gin.TestMode)
	var calls atomic.Int32
	failR := gin.New()
	failR.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, userID)
		c.Next()
	})
	failR.POST("/op", store.Middleware(), func(c *gin.Context) {
		calls.Add(1)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad"})
	})
	failReq := httptest.NewRequest(http.MethodPost, "/op", bytes.NewBufferString(`{"qty":0}`))
	failReq.Header.Set("Content-Type", "application/json")
	failReq.Header.Set(middleware.IdempotencyHeader, "key-fail")
	failRec := httptest.NewRecorder()
	failR.ServeHTTP(failRec, failReq)
	assert.Equal(t, http.StatusBadRequest, failRec.Code)

	// The 400 must not have been persisted; only key-retry's 200 exists.
	require.NoError(t, db.Model(&middleware.IdempotencyKey{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "failed attempt not stored")
}

func TestIdempotencyExpiredRowNotReplayed(t *testing.T) {
	db, userID := setupIdempotencyDB(t)
	store := middleware.NewIdempotencyStore(db)

	// Insert a pre-expired row for the key so the middleware must treat the
	// request as fresh instead of replaying the stale response.
	require.NoError(t, db.Create(&middleware.IdempotencyKey{
		KeyHash:        "expired-hash",
		UserID:         userID,
		Endpoint:       "/op",
		RequestHash:    "stale",
		ResponseStatus: http.StatusOK,
		ResponseBody:   `{"echo":"stale"}`,
		ExpiresAt:      time.Now().Add(-time.Hour),
	}).Error)

	r, calls := buildIdemEngine(t, store, userID)

	w := doIdemReq(r, "key-expired", `{"qty":7}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(1), calls.Load(), "expired row re-executes the handler")
	assert.NotContains(t, w.Body.String(), "stale")
	assert.Contains(t, w.Body.String(), "qty")
}