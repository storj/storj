# Notes for repository split

## Order of changes

1. Move `cmd/internal/wizard` into `uplink/wizard` (or `private/wizard`).
2. Move `s3-benchmark` to a separate repository.
3. Move `gateway` to a separate repository. We don't want miniogw as a `go.mod` dependency in `uplink`.
4. Move `linksharing` to a separate repository.

## TODO

* Create a repository for all our testing, such that the jenkinsfile could be in sync or shared. Otherwise we need to manually keep staticcheck, go, golint etc. settings in sync.
* Repository for creating builds and docker images.

## Potential Problems

### API breaking changes

We cannot make any backwards incompatible changes directly. This means we need to do these in several steps.

1. Add new API.
2. Update all other repositories to use new API.
3. Deprecate old API.

#### Example

Lets say we want to change from `Upload(name string, data []byte)` to `Upload(name string, r io.Reader)`.

PR 1. Add new API:

```
+ UploadStream(name string, r io.Reader)
```

PR 2\*. Change other repositories:

```
- Upload(name, data)
+ UploadStream(name, bytes.NewReader(data))
```

PR 3. Remove from original repository:

```
- UploadStream(name string, data []byte)
```
