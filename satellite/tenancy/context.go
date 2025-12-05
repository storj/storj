// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package tenancy

import (
	"context"
	"strings"
)

// Context contains tenant information for a request.
type Context struct {
	// TenantID identifies which tenant (customer vs Storj) this request belongs to.
	// Empty string ("") represents the Storj tenant (default).
	TenantID string
}

// tenancyKey is the context key for tenant Context.
type tenancyKey struct{}

// contextKey is the singleton instance of tenancyKey used for context storage.
var contextKey = tenancyKey{}

// defaultContext is the default tenant context (Storj tenant).
var defaultContext = &Context{TenantID: ""}

// FromHostname determines the tenant ID based on the request hostname.
func FromHostname(hostname string, lookupMap map[string]string) string {
	// Remove port if present.
	host := hostname
	if idx := strings.Index(hostname, ":"); idx != -1 {
		host = hostname[:idx]
	}

	// Check if the hostname matches any configured tenant hostname.
	if tenantID, ok := lookupMap[host]; ok {
		return tenantID
	}

	return ""
}

// WithContext attaches tenant context to the given context.Context.
func WithContext(ctx context.Context, tenantCtx *Context) context.Context {
	if tenantCtx == nil {
		tenantCtx = defaultContext
	}
	return context.WithValue(ctx, contextKey, tenantCtx)
}

// GetContext retrieves tenant context from the given context.Context.
// This function never returns nil - it returns a default Storj tenant context if no tenant context is set.
func GetContext(ctx context.Context) *Context {
	if tenantCtx, ok := ctx.Value(contextKey).(*Context); ok && tenantCtx != nil {
		return tenantCtx
	}
	return defaultContext
}
