package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// PublicCache makes a public GET cacheable: Cache-Control for browsers and
// shared caches, and a strong ETag so an unchanged menu costs a 304 instead
// of the full payload.
func PublicCache(maxAgeSeconds int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			for k, v := range rec.header {
				w.Header()[k] = v
			}
			if rec.status != http.StatusOK {
				w.WriteHeader(rec.status)
				_, _ = w.Write(rec.body.Bytes())
				return
			}
			sum := sha256.Sum256(rec.body.Bytes())
			etag := `"` + hex.EncodeToString(sum[:16]) + `"`
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAgeSeconds))
			if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(rec.body.Bytes())
		})
	}
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header         { return b.header }
func (b *bufferedResponse) WriteHeader(status int)      { b.status = status }
func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }
