package main

import (
	_ "storj.io/storj/lib/uplink"
)

//go:generate go build -o uplink-plugin.so -buildmode plugin

func main() {}
