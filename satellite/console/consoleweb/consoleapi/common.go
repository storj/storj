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
	switch status {
	case http.StatusNoContent:
		return
	case http.StatusInternalServerError:
		log.Error("returning error to client", zap.Int("code", status), zap.Error(err))
	case http.StatusBadRequest:
		log.Debug("returning error to client", zap.Int("code", status), zap.Error(err))
	default:
		log.Info("returning error to client", zap.Int("code", status), zap.Error(err))
	}

	w.WriteHeader(status)

	err = json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
	if err != nil {
		log.Error("failed to write json error response", zap.Error(ErrUtils.Wrap(err)))
	}
}
