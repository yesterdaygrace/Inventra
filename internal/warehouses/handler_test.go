package warehouses

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

	"inventory/internal/shared/audit"
	sharederr "inventory/internal/shared/errors"
	"inventory/internal/shared/middleware"
	"inventory/internal/shared/validator"
)

type fakeParser struct {
	userID uuid.UUID
	role   string
	err    error
}

func (p fakeParser) ParseAccessToken(string) (uuid.UUID, string, error) {
	return p.userID, p.role, p.err
}

func setupWarehouseEngine(repo Repository, parser middleware.ClaimsParser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService(repo)
	h := NewHandler(svc, validator.New())
	r := gin.New()
	group := r.Group("/api/v1")
	RegisterRoutes(group, h, parser)
	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func TestWarehouseListPublicPaginated(t *testing.T) {
	m := new(mockRepo)
	whs := []*Warehouse{{Code: "WH1", Name: "Alpha"}}
	m.On("ListWithInventoryCount", mock.Anything, mock.MatchedBy(func(q ListQuery) bool { return q.Search == "alp" })).
		Return(whs, int64(1), nil)

	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "GET", "/api/v1/warehouses?search=alp", "", "tok")
	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeBody(t, w)
	assert.True(t, body["success"].(bool))
	assert.NotNil(t, body["pagination"])
}

func TestWarehouseListIsPublicRead(t *testing.T) {
	m := new(mockRepo)
	m.On("ListWithInventoryCount", mock.Anything, mock.Anything).Return([]*Warehouse{}, int64(0), nil)

	// No auth token and no parser: GET must still succeed (mirrors category).
	r := setupWarehouseEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/warehouses", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWarehouseCreateRequiresAdmin(t *testing.T) {
	m := new(mockRepo)
	r := setupWarehouseEngine(m, fakeParser{role: "STAFF"})

	w := doReq(t, r, "POST", "/api/v1/warehouses", `{"code":"WH1","name":"Alpha"}`, "tok")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWarehouseCreateUnauthenticated(t *testing.T) {
	m := new(mockRepo)
	r := setupWarehouseEngine(m, fakeParser{err: sharederr.ErrUnauthorized})

	w := doReq(t, r, "POST", "/api/v1/warehouses", `{"code":"WH1","name":"Alpha"}`, "tok")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWarehouseCreateAdminOK(t *testing.T) {
	m := new(mockRepo)
	want := &Warehouse{ID: uuid.New(), Code: "WH1", Name: "Alpha"}
	m.On("Create", mock.Anything, mock.MatchedBy(func(w *Warehouse) bool { return w.Code == "WH1" })).
		Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*Warehouse).ID = want.ID
	})

	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "POST", "/api/v1/warehouses", `{"code":"WH1","name":"Alpha"}`, "tok")

	assert.Equal(t, http.StatusCreated, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, "Alpha", body["data"].(map[string]any)["name"])
}

func TestWarehouseCreateValidation(t *testing.T) {
	m := new(mockRepo)
	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "POST", "/api/v1/warehouses", `{"code":"","name":""}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWarehouseGet(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(&Warehouse{ID: id, Code: "WH1", Name: "Alpha"}, nil)

	r := setupWarehouseEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/warehouses/"+id.String(), "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Alpha", decodeBody(t, w)["data"].(map[string]any)["name"])
}

func TestWarehouseGetNotFound(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(nil, sharederr.ErrNotFound)

	r := setupWarehouseEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/warehouses/"+id.String(), "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWarehouseUpdateAdminOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(&Warehouse{ID: id, Code: "WH1", Name: "Old"}, nil)
	m.On("Update", mock.Anything, mock.MatchedBy(func(w *Warehouse) bool { return w.Name == "New" })).
		Return(nil)

	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/warehouses/"+id.String(), `{"name":"New"}`, "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, "New", body["data"].(map[string]any)["name"])
}

func TestWarehouseUpdateRejectsEmptyPayload(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})

	w := doReq(t, r, "PUT", "/api/v1/warehouses/"+id.String(), `{}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWarehouseDeleteAdminOK(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountInventoryFor", mock.Anything, id).Return(int64(0), nil)
	m.On("Delete", mock.Anything, id).Return(nil)

	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/warehouses/"+id.String(), "", "tok")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "warehouse deactivated", decodeBody(t, w)["message"])
}

func TestWarehouseDeleteConflictWhenReferenced(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("CountInventoryFor", mock.Anything, id).Return(int64(3), nil)

	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/warehouses/"+id.String(), "", "tok")

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestWarehouseCreateWithDescription(t *testing.T) {
	m := new(mockRepo)
	wanted := &Warehouse{ID: uuid.New(), Code: "WH2", Name: "Alpha"}
	m.On("Create", mock.Anything, mock.MatchedBy(func(w *Warehouse) bool {
		return w.Code == "WH2" && w.Description != nil && *w.Description == "hub"
	})).Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*Warehouse).ID = wanted.ID
	})

	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	body := `{"code":"WH2","name":"Alpha","description":"hub"}`
	w := doReq(t, r, "POST", "/api/v1/warehouses", body, "tok")
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestWarehouseUpdateAllFields(t *testing.T) {
	m := new(mockRepo)
	id := uuid.New()
	m.On("Get", mock.Anything, id).Return(&Warehouse{ID: id, Code: "OLD", Name: "Old"}, nil)
	m.On("Update", mock.Anything, mock.MatchedBy(func(w *Warehouse) bool {
		return w.Code == "NEW" && w.Name == "Updated"
	})).Return(nil)

	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	body := `{"code":"NEW","name":"Updated"}`
	w := doReq(t, r, "PUT", "/api/v1/warehouses/"+id.String(), body, "tok")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWarehouseGetBadUUID(t *testing.T) {
	m := new(mockRepo)
	r := setupWarehouseEngine(m, nil)
	w := doReq(t, r, "GET", "/api/v1/warehouses/not-a-uuid", "", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWarehouseUpdateBadUUID(t *testing.T) {
	m := new(mockRepo)
	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "PUT", "/api/v1/warehouses/not-a-uuid", `{"name":"X"}`, "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWarehouseDeleteBadUUID(t *testing.T) {
	m := new(mockRepo)
	r := setupWarehouseEngine(m, fakeParser{role: "ADMIN"})
	w := doReq(t, r, "DELETE", "/api/v1/warehouses/not-a-uuid", "", "tok")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

type recordingRecorder struct {
	entries []audit.Entry
}

func (r *recordingRecorder) Record(e audit.Entry) {
	r.entries = append(r.entries, e)
}

func TestWarehouseSetAuditRecordsMutations(t *testing.T) {
	m := new(mockRepo)
	m.On("Create", mock.Anything, mock.Anything).Return(nil)

	rec := &recordingRecorder{}
	svc := NewService(m)
	h := NewHandler(svc, validator.New())
	h.SetAudit(rec)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api/v1")
	RegisterRoutes(group, h, fakeParser{role: "ADMIN", userID: uuid.New()})

	w := doReq(t, r, "POST", "/api/v1/warehouses", `{"code":"WH9","name":"Nine"}`, "tok")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Len(t, rec.entries, 1, "mutation should be recorded")
	assert.Equal(t, "warehouse", rec.entries[0].EntityType)
	assert.Equal(t, "CREATE", rec.entries[0].Action)
}

func TestWarehouseSetAuditNilSafe(t *testing.T) {
	m := new(mockRepo)
	svc := NewService(m)
	h := NewHandler(svc, validator.New())
	h.SetAudit(nil) // must not panic
	assert.NotNil(t, h)
}
