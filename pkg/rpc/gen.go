// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
//
// This code generates the compat_drpc and compat_grpc files by reading in
// protobuf definitions. Its purpose is to generate a bunch of type aliases
// and forwarding functions so that a build tag transparently swaps out the
// concrete implementations of the rpcs.

// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/zeebo/errs"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func usage() error {
	return errs.New("usage: %s <dir> <drpc|grpc> <output file>", os.Args[0])
}

func run() error {
	if len(os.Args) < 4 {
		return usage()
	}
	clients, err := findClientsInDir(os.Args[1])
	if err != nil {
		return errs.Wrap(err)
	}
	info, ok := infos[os.Args[2]]
	if !ok {
		return usage()
	}
	return generate(clients, info, os.Args[3])
}

//
// info about the difference between generated files
//

type generateInfo struct {
	Name   string
	Import string
	Prefix string
	Conn   string
	Tag    string
}

var infos = map[string]generateInfo{
	"drpc": {
		Name:   "drpc",
		Import: "storj.io/drpc/drpcconn",
		Prefix: "DRPC",
		Conn:   "drpcconn.Conn",
		Tag:    "drpc",
	},
	"grpc": {
		Name:   "grpc",
		Import: "google.golang.org/grpc", // the saddest newline
		Prefix: "",
		Conn:   "grpc.ClientConn",
		Tag:    "!drpc",
	},
}

//
// main code to generate a compatability file
//

func generate(clients []string, info generateInfo, output string) (err error) {
	var buf bytes.Buffer
	p := printer{w: &buf}
	P := p.P
	Pf := p.Pf

	P("// Copyright (C) 2019 Storj Labs, Inc.")
	P("// See LICENSE for copying information.")
	P()
	P("// +build", info.Tag)
	P()
	P("package rpc")
	P()
	P("import (")
	Pf("%q", info.Import)
	if !strings.HasPrefix(info.Import, "storj.io/") {
		P()
	}
	Pf("%q", "storj.io/storj/pkg/pb")
	P(")")
	P()
	P("// RawConn is a type alias to a", info.Name, "client connection")
	P("type RawConn =", info.Conn)
	P()
	P("type (")
	for _, client := range clients {
		P("//", client, "is an alias to the", info.Name, "client interface")
		Pf("%s = pb.%s%s", client, info.Prefix, client)
		P()
	}
	P(")")
	for _, client := range clients {
		P()
		Pf("// New%s returns the %s version of a %s", client, info.Name, client)
		Pf("func New%s(rc *RawConn) %s {", client, client)
		Pf("return pb.New%s%s(rc)", info.Prefix, client)
		P("}")
		P()
		Pf("// %s returns a %s for this connection", client, client)
		Pf("func (c *Conn) %s() %s {", client, client)
		Pf("return New%s(c.raw)", client)
		P("}")
	}

	if err := p.Err(); err != nil {
		return errs.Wrap(err)
	}
	fmtd, err := format.Source(buf.Bytes())
	if err != nil {
		return errs.Wrap(err)
	}
	return errs.Wrap(ioutil.WriteFile(output, fmtd, 0644))
}

//
// hacky code to find all the rpc clients in a go package
//

var clientRegex = regexp.MustCompile("^type (.*Client) interface {$")

func findClientsInDir(dir string) (clients []string, err error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.pb.go"))
	if err != nil {
		return nil, errs.Wrap(err)
	}
	for _, file := range files {
		fileClients, err := findClientsInFile(file)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		clients = append(clients, fileClients...)
	}
	sort.Strings(clients)
	return clients, nil
}

func findClientsInFile(file string) (clients []string, err error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		switch client := clientRegex.FindSubmatch(line); {
		case client == nil:
		case bytes.HasPrefix(client[1], []byte("DRPC")):
		case bytes.Contains(client[1], []byte("_")):
		default:
			clients = append(clients, string(client[1]))
		}
	}
	return clients, nil
}

//
// helper to check errors while printing
//

type printer struct {
	w   io.Writer
	err error
}

func (p *printer) P(args ...interface{}) {
	if p.err == nil {
		_, p.err = fmt.Fprintln(p.w, args...)
	}
}

func (p *printer) Pf(format string, args ...interface{}) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format+"\n", args...)
	}
}

func (p *printer) Err() error {
	return p.err
}
