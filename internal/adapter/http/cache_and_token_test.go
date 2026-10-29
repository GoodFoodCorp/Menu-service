package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestPublicCacheSetsETagAndAnswers304(t *testing.T) {
	h := PublicCache(30)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"dish-1"}]`))
	}))

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/menu", nil))
	etag := first.Header().Get("ETag")
	if first.Code != http.StatusOK || etag == "" || first.Header().Get("Cache-Control") != "public, max-age=30" {
		t.Fatalf("first: %d %q %q", first.Code, etag, first.Header().Get("Cache-Control"))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/menu", nil)
	req.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	h.ServeHTTP(second, req)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 {
		t.Fatalf("second: %d, %d bytes", second.Code, second.Body.Len())
	}
}

func TestPublicCacheLeavesErrorsUncached(t *testing.T) {
	h := PublicCache(30)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusBadRequest || rec.Header().Get("ETag") != "" {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("ETag"))
	}
}

func TestManagementRoutesRefuseNonAccessTokens(t *testing.T) {
	h := newTestRouter()
	for name, claims := range map[string]jwt.MapClaims{
		"refresh":  {"typ": "refresh"},
		"untyped":  {"typ": nil},
		"audience": {"aud": "elsewhere"},
	} {
		c := jwt.MapClaims{"sub": "user-1", "tenant_id": restaurantA, "role_slugs": []string{"manager"},
			"typ": "access", "iss": "goodfood-auth", "aud": "goodfood-api", "exp": time.Now().Add(time.Hour).Unix()}
		for k, v := range claims {
			if v == nil {
				delete(c, k)
			} else {
				c[k] = v
			}
		}
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(testJWTSecret))
		rec := do(h, http.MethodGet, "/api/menu/manage", "", map[string]string{"Authorization": "Bearer " + tok})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s token: got %d", name, rec.Code)
		}
	}
}
