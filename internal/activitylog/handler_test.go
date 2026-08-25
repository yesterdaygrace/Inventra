package activitylog

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	"inventory/internal/auth"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/validator"
)

type fakeParser struct {
	userID uuid.UUID
	role   string
	perms  []string
	err    error
}

func (p fakeParser) ParseAccessToken(string) (uuid.UUID, string, []string, error) {
	if p.perms != nil {
		return p.userID, p.role, p.perms, p.err
	}
	return p.userID, p.role, auth.PermissionSetForRole(p.role), p.err
}

func setupEngine(repo Repository, parser middleware.ClaimsParser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService(repo, zap.NewNop())
	h := NewHandler(svc, validator.New())
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1"), h, parser)
	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBuffer(nil))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestListNonAdminForbidden(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{role: "STAFF"})

	w := doReq(t, r, "GET", "/api/v1/activity-logs", "tok")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListUnauthenticated(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{err: sharederr.ErrUnauthorized})

	w := doReq(t, r, "GET", "/api/v1/activity-logs", "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAdminOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	details := datatypes.JSON([]byte(`{"name":"Widget"}`))
	eid := "prod-1"
	m.On("List", mock.Anything, mock.MatchedBy(func(q Query) bool {
		return q.EntityType == "product"
	})).Return([]*ActivityLog{{
		ID: id, Action: "CREATE", EntityType: "product", EntityID: &eid, Details: &details, IP: &eid,
	}}, int64(1), nil)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "GET", "/api/v1/activity-logs?entity_type=product", "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.Contains(t, body, "data")
	assert.NotNil(t, body["meta"])
	data := body["data"].([]any)
	assert.Equal(t, "CREATE", data[0].(map[string]any)["action"])
}

func TestListInvalidUserIDFilter(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "GET", "/api/v1/activity-logs?user_id=not-a-uuid", "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListInvalidFromFilter(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "GET", "/api/v1/activity-logs?from=not-a-time", "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListFromAfterToRejected(t *testing.T) {
	m := new(mockRepo)
	r := setupEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "GET", "/api/v1/activity-logs?from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z", "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListEnrichedResponseFields(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	uid := uuid.New()
	before := datatypes.JSON(`{"quantity":10}`)
	after := datatypes.JSON(`{"quantity":15}`)
	reason := "restock"
	ua := "qa-agent/1.0"
	rid := "req-xyz"
	u := &auth.User{ID: uid, Name: "Ada"}
	m.On("List", mock.Anything, mock.Anything).Return([]*ActivityLog{{
		ID: id, UserID: &uid, Action: "STOCK_IN", EntityType: "inventory",
		User: u, BeforeData: &before, AfterData: &after,
		Reason: &reason, UserAgent: &ua, RequestID: &rid,
	}}, int64(1), nil)

	r := setupEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "GET", "/api/v1/activity-logs", "tok")
	assert.Equal(t, http.StatusOK, w.Code)

	data := decode(t, w)["data"].([]any)[0].(map[string]any)
	assert.Equal(t, "Ada", data["user_name"])
	assert.Equal(t, "restock", data["reason"])
	assert.Equal(t, "qa-agent/1.0", data["user_agent"])
	assert.Equal(t, "req-xyz", data["request_id"])
	afterData := data["after_data"].(map[string]any)
	assert.Equal(t, float64(15), afterData["quantity"])
}

var _ Repository = (*mockRepo)(nil)
