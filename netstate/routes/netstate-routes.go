// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"storj.io/storj/storage/boltdb"
)

// NetStateRoutes maintains access to a boltdb client and zap logger
type NetStateRoutes struct {
	DB     *boltdb.Client
	logger *zap.Logger
}

// Message contains the small value provided by the user to be stored
type Message struct {
	Value string `json:"value"`
}

// NewNetStateRoutes instantiates NetStateRoutes
func NewNetStateRoutes(logger *zap.Logger, db *boltdb.Client) *NetStateRoutes {
	return &NetStateRoutes{
		DB:     db,
		logger: logger,
	}
}

// Put takes the given path and small value from the user and formats the values
// to be given to boltdb.Put
func (n *NetStateRoutes) Put(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	n.logger.Debug("entering netstate http put")

	givenPath := ps.ByName("path")
	var msg Message

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&msg)
	if err != nil {
		http.Error(w, "bad request: err decoding response", http.StatusBadRequest)
		n.logger.Error("err decoding response", zap.Error(err))
		return
	}

	file := boltdb.File{
		Path:  []byte(givenPath),
		Value: []byte(msg.Value),
	}

	if err := n.DB.Put(file); err != nil {
		http.Error(w, "err putting file", http.StatusInternalServerError)
		n.logger.Error("err putting file", zap.Error(err))
		return
	}

	n.logger.Debug("put to the db: " + givenPath)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "PUT to %s\n", givenPath)
}

// Get takes the given file path from the user and calls the bolt client's Get function
func (n *NetStateRoutes) Get(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	n.logger.Debug("entering netstate http get")

	fileKey := ps.ByName("path")

	fileValue, err := n.DB.Get([]byte(fileKey))
	if err != nil {
		http.Error(w, "err getting file", http.StatusInternalServerError)
		n.logger.Error("err getting file", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = w.Write(fileValue)
	if err != nil {
		n.logger.Error("err writing response", zap.Error(err))
	}
	w.WriteHeader(http.StatusOK)
	n.logger.Debug("response written: " + string(fileValue))
}

// List calls the bolt client's List function and responds with a list of all saved file paths
// or "filekeys"
func (n *NetStateRoutes) List(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	n.logger.Debug("entering netstate http list")

	filePaths, err := n.DB.List()
	if err != nil {
		http.Error(w, "internal error: unable to list paths", http.StatusInternalServerError)
		n.logger.Error("err listing file paths", zap.Error(err))
		return
	}

	var pathList []string
	for _, path := range filePaths {
		pathList = append(pathList, string(path))
	}

	bytes, err := json.Marshal(pathList)
	if err != nil {
		http.Error(w, "internal error: unable to marshal path list", http.StatusInternalServerError)
		n.logger.Error("err marshaling path list", zap.Error(err))
		return
	}

	_, err = w.Write(bytes)
	if err != nil {
		n.logger.Error("err writing response", zap.Error(err))
	}
	w.WriteHeader(http.StatusOK)
	n.logger.Debug("response written: " + strings.Join(pathList, ", "))
}

// Delete takes a given file path and calls the bolt client's Delete function
func (n *NetStateRoutes) Delete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	n.logger.Debug("entering netstate http delete")

	fileKey := ps.ByName("path")
	if err := n.DB.Delete([]byte(fileKey)); err != nil {
		http.Error(w, "internal error: unable to delete file", http.StatusInternalServerError)
		n.logger.Error("err deleting file", zap.Error(err))
		return
	}
	n.logger.Debug("deleted file: " + fileKey)
	w.WriteHeader(204)
	fmt.Fprintf(w, "Deleted file key: %s", fileKey)
}
