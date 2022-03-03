// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// HTTPError holds http error entity with http status and error itself.
type HTTPError struct {
	Status int
	Err    error
}

// Error returns http error's string representation.
func (e HTTPError) Error() string {
	return e.Err.Error()
}

// ServeError writes JSON error to response output stream.
func ServeError(log *zap.Logger, w http.ResponseWriter, status int, err error) {
	msg := err.Error()
	fields := []zap.Field{
		zap.Int("code", status),
		zap.String("message", msg),
		zap.Error(err),
	}

	if status == http.StatusNoContent {
		return
	} else if status/100 == 5 { // Check for 5XX status.
		log.Error("returning error to client", fields...)
	} else {
		log.Debug("returning error to client", fields...)
	}

	w.WriteHeader(status)

	err = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
	if err != nil {
		log.Debug("failed to write json error response", zap.Error(err))
	}
}
