// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package tenancy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/tenancy"
)

func TestFromHostname(t *testing.T) {
	lookupMap := map[string]string{
		"customer-a.example.com": "customer-a",
		"customer-b.storj.io":    "customer-b",
	}

	tests := []struct {
		name      string
		hostname  string
		lookupMap map[string]string
		want      string
	}{
		{
			name:      "exact match customer-a",
			hostname:  "customer-a.example.com",
			lookupMap: lookupMap,
			want:      "customer-a",
		},
		{
			name:      "exact match customer-b",
			hostname:  "customer-b.storj.io",
			lookupMap: lookupMap,
			want:      "customer-b",
		},
		{
			name:      "hostname with port - strips port",
			hostname:  "customer-a.example.com:8080",
			lookupMap: lookupMap,
			want:      "customer-a",
		},
		{
			name:      "hostname with standard port",
			hostname:  "customer-b.storj.io:443",
			lookupMap: lookupMap,
			want:      "customer-b",
		},
		{
			name:      "unknown hostname returns empty",
			hostname:  "unknown.example.com",
			lookupMap: lookupMap,
			want:      "",
		},
		{
			name:      "localhost returns empty",
			hostname:  "localhost",
			lookupMap: lookupMap,
			want:      "",
		},
		{
			name:      "localhost with port returns empty",
			hostname:  "localhost:10100",
			lookupMap: lookupMap,
			want:      "",
		},
		{
			name:      "nil lookupMap returns empty",
			hostname:  "customer-a.example.com",
			lookupMap: nil,
			want:      "",
		},
		{
			name:      "empty lookupMap returns empty",
			hostname:  "customer-a.example.com",
			lookupMap: map[string]string{},
			want:      "",
		},
		{
			name:      "empty hostname returns empty",
			hostname:  "",
			lookupMap: lookupMap,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tenancy.FromHostname(tt.hostname, tt.lookupMap)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestContextFunctions(t *testing.T) {
	t.Run("WithContext and GetContext roundtrip", func(t *testing.T) {
		testCtx := testcontext.New(t)

		tenantCtx := &tenancy.Context{TenantID: "test-tenant"}
		ctx := tenancy.WithContext(testCtx, tenantCtx)

		retrieved := tenancy.GetContext(ctx)
		require.NotNil(t, retrieved)
		require.Equal(t, "test-tenant", retrieved.TenantID)
	})

	t.Run("GetContext returns default for empty context", func(t *testing.T) {
		testCtx := testcontext.New(t)

		retrieved := tenancy.GetContext(testCtx)
		require.NotNil(t, retrieved)
		require.Equal(t, "", retrieved.TenantID)
	})

	t.Run("WithContext handles nil tenant context", func(t *testing.T) {
		testCtx := testcontext.New(t)

		ctx := tenancy.WithContext(testCtx, nil)

		retrieved := tenancy.GetContext(ctx)
		require.NotNil(t, retrieved)
		require.Equal(t, "", retrieved.TenantID)
	})

	t.Run("GetContext never returns nil", func(t *testing.T) {
		testCtx := testcontext.New(t)

		retrieved := tenancy.GetContext(testCtx)
		require.NotNil(t, retrieved)
	})

	t.Run("Multiple tenant contexts can coexist", func(t *testing.T) {
		ctx1 := tenancy.WithContext(testcontext.New(t), &tenancy.Context{TenantID: "tenant-1"})
		ctx2 := tenancy.WithContext(testcontext.New(t), &tenancy.Context{TenantID: "tenant-2"})

		require.Equal(t, "tenant-1", tenancy.GetContext(ctx1).TenantID)
		require.Equal(t, "tenant-2", tenancy.GetContext(ctx2).TenantID)
	})
}

func TestMiddleware(t *testing.T) {
	lookupMap := map[string]string{
		"customer-a.example.com": "customer-a",
		"customer-b.example.com": "customer-b",
	}

	tests := []struct {
		name           string
		host           string
		lookupMap      map[string]string
		expectedTenant string
	}{
		{
			name:           "customer-a hostname",
			host:           "customer-a.example.com",
			lookupMap:      lookupMap,
			expectedTenant: "customer-a",
		},
		{
			name:           "customer-b hostname",
			host:           "customer-b.example.com",
			lookupMap:      lookupMap,
			expectedTenant: "customer-b",
		},
		{
			name:           "hostname with port",
			host:           "customer-a.example.com:8080",
			lookupMap:      lookupMap,
			expectedTenant: "customer-a",
		},
		{
			name:           "unknown hostname",
			host:           "unknown.example.com",
			lookupMap:      lookupMap,
			expectedTenant: "",
		},
		{
			name:           "localhost",
			host:           "localhost:10100",
			lookupMap:      lookupMap,
			expectedTenant: "",
		},
		{
			name:           "nil lookupMap",
			host:           "customer-a.example.com",
			lookupMap:      nil,
			expectedTenant: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that captures the tenant context.
			var capturedTenantID string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tenantCtx := tenancy.GetContext(r.Context())
				capturedTenantID = tenantCtx.TenantID
				w.WriteHeader(http.StatusOK)
			})

			middleware := tenancy.Middleware(tt.lookupMap)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host

			recorder := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(recorder, req)

			require.Equal(t, tt.expectedTenant, capturedTenantID)
			require.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}
