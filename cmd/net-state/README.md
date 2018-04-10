# Network State Skateboard

### BoltDB Crud Interface

Small program provides a CRUD (create, read, update, delete) interface for paths of small values.
Can store value (i.e. "hello world") at `/my/test/file` and interact with `/my/test/file` through an api, backed by BoltDB.

To run:

```
go run cmd/net-state/main.go
```

Then you can use http methods (Put, Get, List, and Delete) to interact with small values stored on BoltDB.
To store a value to a PUT request body, use the format:
```
{
  "value": "here's my value"
}
```

TODO:
- add zap logger throughout
- replace http routes with grpc + protobufs
