// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package tenancy

import (
	"net/http"

	"github.com/spacemonkeygo/monkit/v3"
)

var mon = monkit.Package()

// Middleware returns an HTTP middleware that extracts the Host header from each request,
// resolves the tenant ID using the WhiteLabel config, and injects the tenant context into the request.
func Middleware(lookupMap map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			ctx := r.Context()

			defer mon.Task()(&ctx)(&err)

			ctx = WithContext(ctx, &Context{
				TenantID: FromHostname(r.Host, lookupMap),
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
