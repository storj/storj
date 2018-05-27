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

	"storj.io/storj/pkg/netstate"
	"storj.io/storj/storage/boltdb"
)

// NetStateRoutes maintains access to a boltdb client and zap logger
type NetStateRoutes struct {
	DB     netstate.DB
	logger *zap.Logger
}

// NewNetStateRoutes instantiates NetStateRoutes
func NewNetStateRoutes(logger *zap.Logger, db netstate.DB) *NetStateRoutes {
	return &NetStateRoutes{
		DB:     db,
		logger: logger,
	}
}

// Start returns a router calling the routes functions
func Start(f *NetStateRoutes) *httprouter.Router {
	router := httprouter.New()

	router.PUT("/pointer/*path", f.Put)
	router.GET("/pointer/*path", f.Get)
	router.GET("/pointer", f.List)
	router.DELETE("/pointer/*path", f.Delete)

	return router
}

// Put takes the given path and pointer from the user, marshals to bytes, and calls boltdb Put
func (n *NetStateRoutes) Put(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	n.logger.Debug("entering netstate http put")

	givenPath := ps.ByName("path")
	var marshalledPointer []byte

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&marshalledPointer)
	if err != nil {
		http.Error(w, "bad request: err decoding response", http.StatusBadRequest)
		n.logger.Error("err decoding response", zap.Error(err))
		return
	}

	pe := boltdb.PointerEntry{
		Path:    []byte(givenPath),
		Pointer: marshalledPointer,
	}

	if err := n.DB.Put(pe); err != nil {
		http.Error(w, "err putting file", http.StatusInternalServerError)
		n.logger.Error("err putting file", zap.Error(err))
		return
	}

	n.logger.Debug("put to the db: " + givenPath)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "PUT to %s\n", givenPath)
}

// Get takes the given path from the user and calls the bolt client's Get function
func (n *NetStateRoutes) Get(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	n.logger.Debug("entering netstate http get")

	pathKey := ps.ByName("path")

	pointerBytes, err := n.DB.Get([]byte(pathKey))
	if err != nil {
		http.Error(w, "err getting file", http.StatusInternalServerError)
		n.logger.Error("err getting file", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = w.Write(pointerBytes)
	if err != nil {
		n.logger.Error("err writing response", zap.Error(err))
	}
	w.WriteHeader(http.StatusOK)
	n.logger.Debug("response written: " + string(pointerBytes))
}

// List calls the bolt client's List function and responds with a list of all saved file paths
// or "filekeys"
func (n *NetStateRoutes) List(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	n.logger.Debug("entering netstate http list")

	pathKeys, err := n.DB.List()
	if err != nil {
		http.Error(w, "internal error: unable to list paths", http.StatusInternalServerError)
		n.logger.Error("err listing path keys", zap.Error(err))
		return
	}

	var pathList []string
	for _, path := range pathKeys {
		pathList = append(pathList, string(path))
	}

	bytes, err := json.Marshal(pathList)
	if err != nil {
		http.Error(w, "internal error: unable to marshal path list", http.StatusInternalServerError)
		n.logger.Error("err marshalling path list", zap.Error(err))
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

	pathKey := ps.ByName("path")
	if err := n.DB.Delete([]byte(pathKey)); err != nil {
		http.Error(w, "internal error: unable to delete pointer entry", http.StatusInternalServerError)
		n.logger.Error("err deleting pointer entry", zap.Error(err))
		return
	}
	n.logger.Debug("deleted pointer entry at path: " + pathKey)
	w.WriteHeader(204)
	fmt.Fprintf(w, "Deleted pointer entry at path: %s", pathKey)
}
