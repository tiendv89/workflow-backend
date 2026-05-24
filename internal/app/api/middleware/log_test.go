package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tiendv89/workflow-backend/internal/app/api/middleware"
)

func TestLog_registers_without_panic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Log(map[string]struct{}{"/healthz": {}}))
	r.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, path := range []string{"/healthz", "/api/test"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("path %s: got %d want 200", path, w.Code)
		}
	}
}

func TestLog_skip_path_does_not_panic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	skipPaths := map[string]struct{}{"/skip": {}}
	r.Use(middleware.Log(skipPaths))
	r.GET("/skip", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/skip", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d want 200", w.Code)
	}
}
