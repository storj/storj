// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package web

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// ServeJSONError writes a JSON error to the response output stream.
func ServeJSONError(log *zap.Logger, w http.ResponseWriter, status int, err error) {
	ServeCustomJSONError(log, w, status, err, err.Error())
}

// ServeCustomJSONError writes a JSON error with a custom message to the response output stream.
func ServeCustomJSONError(log *zap.Logger, w http.ResponseWriter, status int, err error, msg string) {
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
	case http.StatusTooManyRequests:
	default:
		log.Info("returning error to client", fields...)
	}

	w.WriteHeader(status)

	err = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
	if err != nil {
		log.Error("failed to write json error response", zap.Error(err))
	}
}
