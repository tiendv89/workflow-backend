package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/app/api/response"
	"github.com/tiendv89/workflow-backend/internal/domain"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	c.Request = req
	return c, w
}

func newContextWithQuery(query string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/?"+query, nil)
	c.Request = req
	return c, w
}

func TestRespondOK(t *testing.T) {
	c, w := newContext()
	response.RespondOK(c, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(body["success"]) != "true" {
		t.Errorf("expected success=true, got %s", body["success"])
	}
	if _, ok := body["data"]; !ok {
		t.Error("expected data field")
	}
}

func TestRespondError_NotFound(t *testing.T) {
	c, w := newContext()
	se := domain.NewDatabaseNotFound("workspace", "abc")
	response.RespondError(c, se)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code    domain.ErrorCode   `json:"code"`
			Source  domain.ErrorSource `json:"source"`
			Message string             `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Success {
		t.Error("expected success=false")
	}
	if body.Error.Code != domain.ErrDatabaseNotFound {
		t.Errorf("expected ErrDatabaseNotFound, got %s", body.Error.Code)
	}
	if body.Error.Source != domain.ErrorSourceDatabase {
		t.Errorf("expected database source, got %s", body.Error.Source)
	}
}

func TestRespondError_StatusMapping(t *testing.T) {
	cases := []struct {
		code       domain.ErrorCode
		wantStatus int
	}{
		{domain.ErrDatabaseNotFound, http.StatusNotFound},
		{domain.ErrGitHubNotFound, http.StatusNotFound},
		{domain.ErrValidationInvalidURL, http.StatusBadRequest},
		{domain.ErrValidationMissingInput, http.StatusBadRequest},
		{domain.ErrValidationInvalidQuery, http.StatusBadRequest},
		{domain.ErrGitHubUnauthorized, http.StatusUnauthorized},
		{domain.ErrGitHubRateLimit, http.StatusTooManyRequests},
		{domain.ErrAdapterTimeout, http.StatusGatewayTimeout},
		{domain.ErrAdapterInternal, http.StatusInternalServerError},
		{domain.ErrDatabaseQuery, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			c, w := newContext()
			response.RespondError(c, domain.SourceError{Code: tc.code})
			if w.Code != tc.wantStatus {
				t.Errorf("code %s: expected %d, got %d", tc.code, tc.wantStatus, w.Code)
			}
		})
	}
}

func TestRespondValidationError(t *testing.T) {
	c, w := newContext()
	response.RespondValidationError(c, domain.ErrValidationInvalidQuery, "bad param")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var body struct {
		Error struct {
			Code    domain.ErrorCode   `json:"code"`
			Source  domain.ErrorSource `json:"source"`
			Message string             `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Error.Code != domain.ErrValidationInvalidQuery {
		t.Errorf("expected ErrValidationInvalidQuery, got %s", body.Error.Code)
	}
	if body.Error.Source != domain.ErrorSourceValidation {
		t.Errorf("expected validation source, got %s", body.Error.Source)
	}
	if body.Error.Message != "bad param" {
		t.Errorf("expected message %q, got %q", "bad param", body.Error.Message)
	}
}

func TestParsePagination_Defaults(t *testing.T) {
	c, _ := newContextWithQuery("")
	page, limit, ok := response.ParsePagination(c)
	if !ok {
		t.Fatal("expected ok=true for empty query")
	}
	if page != 1 {
		t.Errorf("expected default page=1, got %d", page)
	}
	if limit != 0 {
		t.Errorf("expected default limit=0, got %d", limit)
	}
}

func TestParsePagination_ValidValues(t *testing.T) {
	c, _ := newContextWithQuery("page=3&limit=20")
	page, limit, ok := response.ParsePagination(c)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if page != 3 {
		t.Errorf("expected page=3, got %d", page)
	}
	if limit != 20 {
		t.Errorf("expected limit=20, got %d", limit)
	}
}

func TestParsePagination_InvalidPage(t *testing.T) {
	c, w := newContextWithQuery("page=abc")
	_, _, ok := response.ParsePagination(c)
	if ok {
		t.Error("expected ok=false for non-integer page")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestParsePagination_PageLessThanOne(t *testing.T) {
	c, w := newContextWithQuery("page=0")
	_, _, ok := response.ParsePagination(c)
	if ok {
		t.Error("expected ok=false for page=0")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestParsePagination_InvalidLimit(t *testing.T) {
	c, w := newContextWithQuery("limit=xyz")
	_, _, ok := response.ParsePagination(c)
	if ok {
		t.Error("expected ok=false for non-integer limit")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
