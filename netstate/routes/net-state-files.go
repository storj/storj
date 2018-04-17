// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/zeebo/errs"

	"github.com/storj/storage/boltdb"
)

var (
	// Error is the default route errs class
	Error      = errs.Class("routes err")
	errReadReq = Error.New("error reading request body")
)

type NetStateRoutes struct {
	DB *boltdb.Client
}

type Message struct {
	Value string `json:"value"`
}

func NewNetStateRoutes(db *boltdb.Client) *NetStateRoutes {
	return &NetStateRoutes{DB: db}
}

func (n *NetStateRoutes) Put(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	givenPath := ps.ByName("path")

	var msg Message
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&msg)
	if err != nil {
		http.Error(w, "bad request: err decoding response", http.StatusBadRequest)
		log.Printf("err decoding response: %v", err)
		return
	}

	file := boltdb.File{
		Path:  givenPath,
		Value: msg.Value,
	}

	if err := n.DB.Put(file); err != nil {
		http.Error(w, "err saving file", http.StatusInternalServerError)
		log.Println(err)
	}

	fmt.Fprintf(w, "PUT to %s\n", givenPath)
}

func (n *NetStateRoutes) Get(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fileKey := ps.ByName("path")

	fileInfo, err := n.DB.Get([]byte(fileKey))
	if err != nil {
		log.Println(err)
	}

	bytes, err := json.Marshal(fileInfo)
	if err != nil {
		http.Error(w, "internal error: unable to get value", http.StatusInternalServerError)
		return
	}
	_, err = w.Write(bytes)
	if err != nil {
		log.Printf("failed writing response: %v", err)
	}
}

func (n *NetStateRoutes) List(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fileKeys, err := n.DB.List()
	if err != nil {
		http.Error(w, "internal error: unable to list paths", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	keyString := strings.Join(fileKeys, "")
	_, err = w.Write([]byte(keyString))
	if err != nil {
		log.Printf("failed writing response: %v", err)
	}
}

func (n *NetStateRoutes) Delete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fileKey := ps.ByName("path")
	if err := n.DB.Delete([]byte(fileKey)); err != nil {
		http.Error(w, "internal error: unable to delete file", http.StatusInternalServerError)
		log.Printf("err deleting file %v", err)
	}

	fmt.Fprintf(w, "Deleted file key: %s", fileKey)
}
