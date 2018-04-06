package main

import (
	"fmt"

	"github.com/kataras/iris"
	"github.com/storj/storj/routes"
	"github.com/storj/storj/storage/boltdb"
)

func main() {

	bdb, err := boltdb.New()
	if err != nil {
		fmt.Println(err)
		return
	}

	defer bdb.DB.Close()

	users := routes.Users{DB: bdb}
	app := iris.Default()

	SetRoutes(app, users)

	app.Run(iris.Addr(":8080"))
}

// SetRoutes defines all restful routes on the service
func SetRoutes(app *iris.Application, users routes.Users) {
	app.Post("/users/:id", users.CreateUser)
	app.Get("/users/:id", users.GetUser)
	app.Put("/users/:id/:email", users.UpdateUser)
	app.Delete("/users/:id", users.DeleteUser)
	// app.Get("/users/confirmations/:token", users.Confirm)
	// app.Get("/files?startDate=<timestamp>?tag=<tag>", files.ListFiles)
	// app.Get("/file-ids/:name", files.GetFileId)
	// app.Get("/files/:file?skip=<number>&limit=<number>&exclude=<node-ids>", files.GetPointers)
	// app.Delete("/files/:file", files.DeleteFile)
	// app.Post("/files", files.NewFile)
	// app.Put("/files/:file/shards/:index", files.AddShardToFile)
	// app.Post("/reports", reports.CreateReport)
	// app.Get("/contacts?address=<address>&skip=<number>&limit=<number>", contacts.GetContacts)

}
