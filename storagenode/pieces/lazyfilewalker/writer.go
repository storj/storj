// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"bytes"
	"encoding/json"

	"go.uber.org/zap"

	"storj.io/common/storj"
)

type writer interface {
	Write(p []byte) (n int, err error)
	Decode(v interface{}) error
}

// check that genericWriter and trashHandler implement the writer interface.
var _ writer = (*genericWriter)(nil)
var _ writer = (*TrashHandler)(nil)

// genericWriter is a writer that processes the output of the lazyfilewalker subprocess.
type genericWriter struct {
	buf bytes.Buffer
	log *zap.Logger
}

func newGenericWriter(log *zap.Logger) *genericWriter {
	return &genericWriter{
		log: log,
	}
}

// Decode decodes the data from the buffer into the provided value.
func (w *genericWriter) Decode(v interface{}) error {
	if err := json.NewDecoder(&w.buf).Decode(&v); err != nil {
		w.log.Error("failed to decode response from subprocess", zap.Error(err))
		return err
	}
	return nil
}

// Write writes the provided bytes to the buffer.
func (w *genericWriter) Write(b []byte) (n int, err error) {
	return w.buf.Write(b)
}

// TrashHandler is a writer that processes the output of the gc-filewalker subprocess.
type TrashHandler struct {
	buf        *genericWriter
	log        *zap.Logger
	lineBuffer []byte

	trashFunc func(pieceID storj.PieceID) error
}

// NewTrashHandler creates new trash handler.
func NewTrashHandler(log *zap.Logger, trashFunc func(pieceID storj.PieceID) error) *TrashHandler {
	return &TrashHandler{
		log:       log.Named("trash-handler"),
		trashFunc: trashFunc,
		buf:       newGenericWriter(log),
	}
}

// Decode decodes the data from the buffer into the provided value.
func (t *TrashHandler) Decode(v interface{}) error {
	return t.buf.Decode(v)
}

// Write writes the provided bytes to the buffer.
func (t *TrashHandler) Write(b []byte) (n int, err error) {
	n = len(b)
	t.lineBuffer = append(t.lineBuffer, b...)
	for {
		if b, err = t.writeLine(t.lineBuffer); err != nil {
			return n, err
		}
		if len(b) == len(t.lineBuffer) {
			break
		}

		t.lineBuffer = b
	}

	return n, nil
}

func (t *TrashHandler) writeLine(b []byte) (remaining []byte, err error) {
	idx := bytes.IndexByte(b, '\n')
	if idx < 0 {
		return b, nil
	}

	b, remaining = b[:idx], b[idx+1:]

	return remaining, t.processTrashPiece(b)
}

func (t *TrashHandler) processTrashPiece(b []byte) error {
	var resp GCFilewalkerResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		t.log.Error("failed to unmarshal data from subprocess", zap.Error(err))
		return err
	}

	if !resp.Completed {
		for _, pieceID := range resp.PieceIDs {
			t.log.Debug("trashing piece", zap.Stringer("pieceID", pieceID))
			if err := t.trashFunc(pieceID); err != nil {
				return err
			}

		}
		return nil
	}

	_, err := t.buf.Write(b)
	return err
}
