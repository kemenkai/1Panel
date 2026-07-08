package security

import (
	"testing"
)

func TestIsFrontendPathRecognizesEnhanceRoutes(t *testing.T) {
	paths := []string{
		"/enhance",
		"/enhance/home",
		"/enhance/windows-service",
	}

	for _, path := range paths {
		if !IsFrontendPath(path) {
			t.Fatalf("expected %s to be treated as frontend route", path)
		}
	}
}
