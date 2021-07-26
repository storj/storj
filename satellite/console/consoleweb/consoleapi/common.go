// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

var (
	// ErrUtils - console utils error type.
	ErrUtils = errs.Class("console api utils")
)

// serveJSONError writes a JSON error to the response output stream.
func serveJSONError(log *zap.Logger, w http.ResponseWriter, status int, err error) {
	serveCustomJSONError(log, w, status, err, err.Error())
}

// serveCustomJSONError writes a JSON error with a custom message to the response output stream.
func serveCustomJSONError(log *zap.Logger, w http.ResponseWriter, status int, err error, msg string) {
	fields := []zap.Field{
		zap.Int("code", status),
		zap.String("message", msg),
		zap.Error(err),
	}
	switch status {
	case http.StatusNoContent:
		return
	case http.StatusInternalServerError:
		log.Error("returning error to client", fields...)
	case http.StatusBadRequest:
		log.Debug("returning error to client", fields...)
	default:
		log.Info("returning error to client", fields...)
	}

	w.WriteHeader(status)

	err = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
	if err != nil {
		log.Error("failed to write json error response", zap.Error(ErrUtils.Wrap(err)))
	}
}
