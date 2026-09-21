// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package mailservice

import "github.com/zeebo/errs"

// TemplateFuncs provides the shared functions for HTML and plain-text email templates.
var TemplateFuncs = map[string]any{
	"dict": templateDict,
}

// templateDict builds a template context from alternating string keys and values.
func templateDict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, errs.New("dict requires key/value pairs")
	}
	result := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, errs.New("dict key at argument %d must be a string", i+1)
		}
		result[key] = pairs[i+1]
	}
	return result, nil
}
