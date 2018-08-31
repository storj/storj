// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"go.uber.org/zap"

	"storj.io/storj/storage"
	"storj.io/storj/storage/storelogger"
	"storj.io/storj/storage/teststore"
)

func main() {
	rawstore := teststore.New()
	lg, _ := zap.NewDevelopment()
	store := storelogger.New(lg, rawstore)

	store.Put(storage.Key("a"), storage.Value("a"))
	store.Put(storage.Key("b/1"), storage.Value("b/1"))
	store.Put(storage.Key("b/2"), storage.Value("b/2"))
	store.Put(storage.Key("b/3"), storage.Value("b/3"))
	store.Put(storage.Key("c"), storage.Value("c"))
	store.Put(storage.Key("c/"), storage.Value("c/"))
	store.Put(storage.Key("c//"), storage.Value("c//"))
	store.Put(storage.Key("c/1"), storage.Value("c/1"))
	store.Put(storage.Key("g"), storage.Value("g"))
	store.Put(storage.Key("h"), storage.Value("h"))

	/*
		store.IterateReverse(storage.Key("x-"), storage.Key("x-b/2"), '/',
			func(it storage.Iterator) error {
				var item storage.ListItem
				for it.Next(&item) {
					fmt.Printf("%q = %q: %v\n", item.Key, item.Value, item.IsPrefix)
				}
				return nil
			})
	*/

	store.IterateReverse(storage.Key("b/"), storage.Key("b/2"), '/',
		func(it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(&item) {
				fmt.Printf("%q = %q: %v\n", item.Key, item.Value, item.IsPrefix)
			}
			return nil
		})

}
