package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func buildFrontendPathContext(path string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	return ctx
}

func TestIsFrontendPathRecognizesEnhanceRoutes(t *testing.T) {
	t.Helper()

	paths := []string{
		"/enhance",
		"/enhance/home",
		"/enhance/windows-service",
	}

	for _, path := range paths {
		if !isFrontendPath(buildFrontendPathContext(path)) {
			t.Fatalf("expected %s to be treated as frontend route", path)
		}
	}
}
