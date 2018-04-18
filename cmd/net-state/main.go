// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"storj.io/storj/netstate/routes"
	"storj.io/storj/storage/boltdb"
)

func main() {
	err := Main()
	if err != nil {
		log.Fatalf("fatal error: %v", err)
		os.Exit(1)
	}
}

func Main() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	defer logger.Sync()
	logger.Info("serving on localhost:3000")

	bdb, err := boltdb.New("netstate.db")
	if err != nil {
		return err
	}
	defer bdb.Close()

	routes := routes.NewNetStateRoutes(bdb)

	return http.ListenAndServe(":3000", start(routes))
}

func start(f *routes.NetStateRoutes) *httprouter.Router {
	router := httprouter.New()

	router.PUT("/file/*path", f.Put)
	router.GET("/file/*path", f.Get)
	router.GET("/file", f.List)
	router.DELETE("/file/*path", f.Delete)

	return router
}
