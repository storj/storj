// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"github.com/storj/netstate/routes"
	"github.com/storj/storage/boltdb"
)

var (
	// Error is the default main errs class
	Error = errs.Class("main err")

	errLoggerFail = Error.New("zap logger failed to start")
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Println(errLoggerFail)
		return
	}
	defer logger.Sync()
	logger.Info("serving on localhost:3000")

	bdb, err := boltdb.New("netstate.db")
	if err != nil {
		logger.Fatal("db error:",
			zap.Error(boltdb.ErrInitDb),
			zap.Error(err),
		)
		return
	}
	defer bdb.Close()

	file := routes.NewNetStateRoutes(bdb)

	http.ListenAndServe(":3000", start(file))
}

func start(f *routes.NetStateRoutes) *httprouter.Router {
	router := httprouter.New()

	router.PUT("/file/*path", f.Put)
	router.GET("/file/*path", f.Get)
	router.GET("/file", f.List)
	router.DELETE("/file/*path", f.Delete)

	return router
}
