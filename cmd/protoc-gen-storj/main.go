// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"strconv"
	"strings"

	pb "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	"github.com/gogo/protobuf/vanity/command"
)

const (
	invokerMethods = `
		Invoke(ctx context.Context, rpc string, in, out interface{}) error
		NewStream(ctx context.Context, rpc string) (` + streamer + `, error)
		Close() error
	`
	streamerMethods = `
		SendMsg(msg interface{}) error
		RecvMsg(msg interface{}) error
		CloseSend() error
	`
	invoker     = "interface { " + invokerMethods + " }"
	streamer    = "interface { " + streamerMethods + " }"
	handlerType = "func(interface{}, context.Context, interface{}, interface{}) (interface{}, error)"
)

func main() {
	generator.RegisterPlugin(new(drpc))
	command.Write(command.Generate(command.Read()))
}

type drpc struct {
	*generator.Generator

	contextPkg string
}

//
// helpers/boilerplate
//

func (d *drpc) Name() string { return "drpc" }

func (d *drpc) GenerateImports(file *generator.FileDescriptor) {}

func (d *drpc) Init(g *generator.Generator) {
	d.Generator = g
}

func (d *drpc) Pf(format string, args ...interface{}) {
	d.P(fmt.Sprintf(format, args...))
}

func (d *drpc) Generate(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	d.contextPkg = string(d.AddImport("context"))
	for i, service := range file.FileDescriptorProto.Service {
		d.generateService(file, service, i)
	}
}

func (d *drpc) typeName(str string) string {
	return d.TypeName(d.objectNamed(str))
}

func (d *drpc) objectNamed(name string) generator.Object {
	d.RecordTypeUse(name)
	return d.ObjectNamed(name)
}

func unexport(s string) string {
	return strings.ToLower(s[:4]) + s[4:]
}

//
// main generation
//

func (d *drpc) generateService(file *generator.FileDescriptor, service *pb.ServiceDescriptorProto, index int) {
	path := fmt.Sprintf("6,%d", index) // 6 means service.

	fullServName := service.GetName()
	if pkg := file.GetPackage(); pkg != "" {
		fullServName = pkg + "." + fullServName
	}
	servName := "DRPC" + generator.CamelCase(service.GetName())

	// Client interface
	d.P("type ", servName, "Client interface {")
	d.P("Transport() ", invoker)
	for i, method := range service.Method {
		d.PrintComments(fmt.Sprintf("%s,2,%d", path, i))
		d.P(d.generateClientSignature(servName, method))
	}
	d.P("}")
	d.P()

	// Client implementation
	d.P("type ", unexport(servName), "Client struct {")
	d.P("cc ", invoker)
	d.P("}")
	d.P()

	// Client constructor
	d.P("func New", servName, "Client(cc ", invoker, ") ", servName, "Client {")
	d.P("return &", unexport(servName), "Client{cc}")
	d.P("}")
	d.P()

	// Client method implementations
	d.P("func (c *", unexport(servName), "Client) Transport() ", invoker, "{ return c.cc }")
	d.P()
	for _, method := range service.Method {
		d.generateClientMethod(servName, fullServName, method)
	}

	// Server interface
	d.P("type ", servName, "Server interface {")
	for i, method := range service.Method {
		d.PrintComments(fmt.Sprintf("%s,2,%d", path, i))
		d.P(d.generateServerSignature(servName, method))
	}
	d.P("}")
	d.P()

	// Server description.
	d.P("type ", servName, "Description struct{}")
	d.P()
	d.P("func (", servName, "Description) NumMethods() int { return ", len(service.Method), " }")
	d.P()
	d.P("func (", servName, "Description) Method(n int) (string, ", handlerType, ", interface{}, bool) {")
	d.P("switch n {")
	for i, method := range service.Method {
		methName := generator.CamelCase(method.GetName())
		d.P("case ", i, ":")
		d.P("return ", strconv.Quote("/"+fullServName+"/"+method.GetName()), ",")
		d.generateServerHandler(servName, method)
		d.P("}, ", servName, "Server.DRPC", methName, ", true")
	}
	d.P("default:")
	d.P(`return "", nil, nil, false`)
	d.P("}")
	d.P("}")
	d.P()

	// Server methods
	for _, method := range service.Method {
		d.generateServerMethod(servName, fullServName, method)
	}
}

//
// client methods
//

func (d *drpc) generateClientSignature(servName string, method *pb.MethodDescriptorProto) string {
	origMethName := method.GetName()
	methName := generator.CamelCase(origMethName)
	reqArg := ", in *" + d.typeName(method.GetInputType())
	if method.GetClientStreaming() {
		reqArg = ""
	}
	respName := "*" + d.typeName(method.GetOutputType())
	if method.GetServerStreaming() || method.GetClientStreaming() {
		respName = servName + "_" + generator.CamelCase(origMethName) + "Client"
	}
	return fmt.Sprintf("%s(ctx %s.Context%s) (%s, error)", methName, d.contextPkg, reqArg, respName)
}

func (d *drpc) generateClientMethod(servName, fullServName string, method *pb.MethodDescriptorProto) {
	sname := fmt.Sprintf("/%s/%s", fullServName, method.GetName())
	methName := generator.CamelCase(method.GetName())
	inType := d.typeName(method.GetInputType())
	outType := d.typeName(method.GetOutputType())

	d.P("func (c *", unexport(servName), "Client) ", d.generateClientSignature(servName, method), "{")
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		d.P("out := new(", outType, ")")
		d.P("err := c.cc.Invoke(ctx, ", strconv.Quote(sname), ", in, out)")
		d.P("if err != nil { return nil, err }")
		d.P("return out, nil")
		d.P("}")
		d.P()
		return
	}
	streamType := unexport(servName) + methName + "Client"
	d.P("stream, err := c.cc.NewStream(ctx, ", strconv.Quote(sname), ")")
	d.P("if err != nil { return nil, err }")
	d.P("x := &", streamType, "{stream}")
	if !method.GetClientStreaming() {
		d.P("if err := x.stream.SendMsg(in); err != nil { return nil, err }")
		d.P("if err := x.stream.CloseSend(); err != nil { return nil, err }")
	}
	d.P("return x, nil")
	d.P("}")
	d.P()

	genSend := method.GetClientStreaming()
	genRecv := method.GetServerStreaming()
	genCloseAndRecv := !method.GetServerStreaming()

	// Stream auxiliary types and methods.
	d.P("type ", servName, "_", methName, "Client interface {")
	if genSend {
		d.P("Send(*", inType, ") error")
	}
	if genRecv {
		d.P("Recv() (*", outType, ", error)")
	}
	if genCloseAndRecv {
		d.P("CloseAndRecv() (*", outType, ", error)")
	}
	d.P("}")
	d.P()

	d.P("type ", streamType, " struct {")
	d.P("stream ", streamer)
	d.P("}")
	d.P()

	if genSend {
		d.P("func (x *", streamType, ") Send(m *", inType, ") error {")
		d.P("return x.stream.SendMsg(m)")
		d.P("}")
		d.P()
	}
	if genRecv {
		d.P("func (x *", streamType, ") Recv() (*", outType, ", error) {")
		d.P("m := new(", outType, ")")
		d.P("if err := x.stream.RecvMsg(m); err != nil { return nil, err }")
		d.P("return m, nil")
		d.P("}")
		d.P()
	}
	if genCloseAndRecv {
		d.P("func (x *", streamType, ") CloseAndRecv() (*", outType, ", error) {")
		d.P("if err := x.stream.CloseSend(); err != nil { return nil, err }")
		d.P("m := new(", outType, ")")
		d.P("if err := x.stream.RecvMsg(m); err != nil { return nil, err }")
		d.P("return m, nil")
		d.P("}")
		d.P()
	}
}

//
// server methods
//

func (d *drpc) generateServerSignature(servName string, method *pb.MethodDescriptorProto) string {
	methName := generator.CamelCase(method.GetName())

	var reqArgs []string
	ret := "error"
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		reqArgs = append(reqArgs, d.contextPkg+".Context")
		ret = "(*" + d.typeName(method.GetOutputType()) + ", error)"
	}
	if !method.GetClientStreaming() {
		reqArgs = append(reqArgs, "*"+d.typeName(method.GetInputType()))
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		reqArgs = append(reqArgs, servName+"_"+methName+"Stream")
	}

	return "DRPC" + methName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func (d *drpc) generateServerHandler(servName string, method *pb.MethodDescriptorProto) {
	methName := generator.CamelCase(method.GetName())
	streamType := unexport(servName) + methName + "Stream"

	d.P("func (srv interface{}, ctx context.Context, in1, in2 interface{}) (interface{}, error) {")
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		d.P("return srv.(", servName, "Server).")
	} else {
		d.P("return nil, srv.(", servName, "Server).")
	}
	d.P("DRPC", methName, "(")

	n := 1
	if !method.GetServerStreaming() && !method.GetClientStreaming() {
		d.P("ctx,")
	}
	if !method.GetClientStreaming() {
		d.P("in", n, ".(*", d.typeName(method.GetInputType()), "),")
		n++
	}
	if method.GetServerStreaming() || method.GetClientStreaming() {
		d.P("&", streamType, "{in", n, ".(", streamer, ")},")
	}
	d.P(")")
}

func (d *drpc) generateServerMethod(servName, fullServName string, method *pb.MethodDescriptorProto) {
	methName := generator.CamelCase(method.GetName())
	inType := d.typeName(method.GetInputType())
	outType := d.typeName(method.GetOutputType())
	streamType := unexport(servName) + methName + "Stream"

	genSend := method.GetServerStreaming()
	genSendAndClose := !method.GetServerStreaming()
	genRecv := method.GetClientStreaming()

	// Stream auxiliary types and methods.
	d.P("type ", servName, "_", methName, "Stream interface {")
	if genSend {
		d.P("Send(*", outType, ") error")
	}
	if genSendAndClose {
		d.P("SendAndClose(*", outType, ") error")
	}
	if genRecv {
		d.P("Recv() (*", inType, ", error)")
	}
	d.P("}")
	d.P()

	d.P("type ", streamType, " struct {")
	d.P("streamer ", streamer)
	d.P("}")
	d.P()

	if genSend {
		d.P("func (x *", streamType, ") Send(m *", outType, ") error {")
		d.P("return x.streamer.SendMsg(m)")
		d.P("}")
		d.P()
	}
	if genSendAndClose {
		d.P("func (x *", streamType, ") SendAndClose(m *", outType, ") error {")
		d.P("if err := x.streamer.SendMsg(m); err != nil { return err }")
		d.P("return x.streamer.CloseSend()")
		d.P("}")
		d.P()
	}
	if genRecv {
		d.P("func (x *", streamType, ") Recv() (*", inType, ", error) {")
		d.P("m := new(", inType, ")")
		d.P("if err := x.streamer.RecvMsg(m); err != nil { return nil, err }")
		d.P("return m, nil")
		d.P("}")
		d.P()
	}
}
