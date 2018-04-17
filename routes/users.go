// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package routes

import (
	"log"

	"github.com/google/uuid"
	"github.com/kataras/iris"

	"storj.io/storj/storage/boltdb"
)

// Users contains items needed to process requests to the user namespace
type Users struct {
	DB *boltdb.Client
}

func (u *Users) CreateUser(ctx iris.Context) {
	user := boltdb.User{
		Id:       uuid.New(),
		Username: ctx.Params().Get("id"),
		Email:    `dece@trali.zzd`,
	}

	if err := ctx.ReadJSON(user); err != nil {
		ctx.JSON(iris.StatusNotAcceptable)
	}

	u.DB.CreateUser(user)
}

func (u *Users) GetUser(ctx iris.Context) {
	userId := ctx.Params().Get("id")
	userInfo, err := u.DB.GetUser([]byte(userId))
	if err != nil {
		log.Println(err)
	}

	ctx.Writef("%s's info is: %s", userId, userInfo)
}

// Updates only email for now
// Uses two db queries now, can refactor
func (u *Users) UpdateUser(ctx iris.Context) {
	userId := ctx.Params().Get("id")
	userInfo, err := u.DB.GetUser([]byte(userId))
	if err != nil {
		log.Println(err)
	}

	updated := boltdb.User{
		Id:       userInfo.Id,
		Username: userInfo.Username,
		Email:    ctx.Params().Get("email"),
	}

	err1 := u.DB.UpdateUser(updated)
	if err1 != nil {
		log.Println(err)
	}
}

func (u *Users) DeleteUser(ctx iris.Context) {
	userId := ctx.Params().Get("id")
	u.DB.DeleteUser([]byte(userId))
}
