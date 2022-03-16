// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package web

import (
	"net/http"
	"path"
	"strings"
)

// CacheHandler is a middleware for caching static files for 1 year.
func CacheHandler(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()

		// "mime" package, which http.FileServer uses, depends on Operating System
		// configuration for mime-types. When a system has hardcoded mime-types to
		// something else, they might serve ".js" as a "plain/text".
		//
		// Override any of that default behavior to ensure we get the correct types for
		// common files.
		if contentType, ok := CommonContentType(path.Ext(r.URL.Path)); ok {
			header.Set("Content-Type", contentType)
		}

		header.Set("Cache-Control", "public, max-age=31536000")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "same-origin")

		fn.ServeHTTP(w, r)
	})
}

// CommonContentType returns content-type for common extensions,
// ignoring OS settings.
func CommonContentType(ext string) (string, bool) {
	ext = strings.ToLower(ext)
	mime, ok := commonContentType[ext]
	return mime, ok
}

var commonContentType = map[string]string{
	".css":   "text/css; charset=utf-8",
	".gif":   "image/gif",
	".htm":   "text/html; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".jpeg":  "image/jpeg",
	".jpg":   "image/jpeg",
	".js":    "application/javascript",
	".mjs":   "application/javascript",
	".otf":   "font/otf",
	".pdf":   "application/pdf",
	".png":   "image/png",
	".svg":   "image/svg+xml",
	".ttf":   "font/ttf",
	".wasm":  "application/wasm",
	".webp":  "image/webp",
	".xml":   "text/xml; charset=utf-8",
	".sfnt":  "font/sfnt",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}
