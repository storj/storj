package main

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/storj/routes"
	"github.com/storj/storage/boltdb"

	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Println("zap logger failed to start")
		return
	}
	defer logger.Sync()
	logger.Info("serving on localhost:3000")

	bdb, err := boltdb.New("test.db")
	if err != nil {
		logger.Fatal("db error:",
			zap.Error(boltdb.ErrInitDb),
			zap.Error(err),
		)
		return
	}
	defer bdb.Close()

	file := routes.NewFile(bdb)

	http.ListenAndServe(":3000", start(file))
}

func start(f *routes.File) *httprouter.Router {
	router := httprouter.New()

	router.PUT("/file/*path", f.Put)
	router.GET("/file/*path", f.Get)
	router.GET("/file", f.List)
	router.DELETE("/file/*path", f.Delete)

	return router
}
