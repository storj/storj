# HTTP + BoltDB Crud Interface

This is an http server that provides a CRUD (create, read, update, delete) interface for storing file paths and small values with BoltDB.
For example, you can store a value (i.e. "hello world") at `/my/test/file` and interact with `/my/test/file` through an API, backed by BoltDB.

To run:
```
go run cmd/netstate-http/main.go
```
You can also run using these flags: `-port=<port-number> -prod=<bool> -db=<db-name>`

Then you can use http methods (Put, Get, List, and Delete) to interact with small values stored on BoltDB.
To store a value, put it into the PUT request body using this format:
```
{
  "value": "here's my value"
}
```
If you're using Postman, select the `raw` request setting to do this.

Afterward, you can also use [Bolter](https://github.com/hasit/bolter) or a similar BoltDB viewer to make sure your files were changed as expected.