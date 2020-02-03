// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import "strings"

// CommonContentType returns content-type for common extensions.
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
