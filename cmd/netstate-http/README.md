# HTTP + BoltDB Crud Interface

This is an http server that provides a CRUD (create, read, update, delete) interface with BoltDB for storing pointers at a given file path.
For example, you can put a pointer at the path `/my/test/file` and interact with `/my/test/file` through an API, backed by BoltDB.

To run:
```
go run cmd/netstate-http/main.go
```
You can also run using these flags: `-port=<port-number> -prod=<bool> -db=<db-name>`

Then you can use http methods (Put, Get, List, and Delete) to interact with pointers stored on BoltDB.
To store a pointer, put it into the PUT request body using this format:
```
{
  "Pointer": {
    "Type": "INLINE",
    "Encryption": {
      "EncryptedEncryptionKey": "key",
      "EncryptedStartingNonce": "nonce"
    },
    "InlineSegment": "test"
  }
}
```
If you're using Postman, select the `raw` request setting to do this.

Afterward, you can also use [Bolter](https://github.com/hasit/bolter) or a similar BoltDB viewer to make sure your files were changed as expected.