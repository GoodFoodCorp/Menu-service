package authclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTenantsCallsRestaurantsPathOnTheServiceRoot(t *testing.T) {
	for _, suffix := range []string{"", "/"} {
		var gotPath, gotMethod string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			_, _ = w.Write([]byte(`[{"id":"50d489cc-4d8b-4726-8c57-dc6d9e04d167","name":"Montparnasse","slug":"montparnasse","isActive":true},` +
				`{"id":"92db2fe8-7988-492c-8fcd-ec81ee7f52d1","name":"Republique","slug":"default","isActive":false}]`))
		}))

		tenants, err := New(srv.URL + suffix).ListTenants(context.Background())
		srv.Close()

		require.NoError(t, err)
		assert.Equal(t, http.MethodGet, gotMethod)
		assert.Equal(t, "/api/restaurants", gotPath, "base URL %q must not produce a double slash", srv.URL+suffix)
		require.Len(t, tenants, 2)
		assert.Equal(t, "50d489cc-4d8b-4726-8c57-dc6d9e04d167", tenants[0].ID)
		assert.True(t, tenants[0].IsActive)
		assert.False(t, tenants[1].IsActive)
	}
}

func TestListTenantsReturnsAnErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()

	_, err := New(srv.URL).ListTenants(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "franchise-service returned 404")
}
