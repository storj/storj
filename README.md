# Notes for repository split

## Order of changes

1. Kill `storj.io/storj/storage` for `storj.io/storj/storj/uplink`

2. Move `storj.io/storj/private/*` to `storj.io/core/*`.
3. Move `storj.io/storj/pkg/*` to `storj.io/core/*` (only uplink dependencies).
4. Move `storj.io/storj/uplink/*` to `storj.io/uplink/*`

2. Move `s3-benchmark` to a separate repository.
2. Move `gateway` to a separate repository. We don't want miniogw as a `go.mod` dependency in `uplink`.
2. Move `linksharing` to a separate repository.

5. Move `cmd/internal/wizard` into `uplink/wizard` (or `private/wizard`), maybe just later.

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

## Thoughts on CI

### Single Jenkinsfile

1. pull all repositories
2. update dependency to the current repository
3. run all tests

```
ci/ [repository]
    linter/*
    Dockerfile
    Jenkinsfile
        def Everything():
            pull core, satellite, storagenode
            run tests

xyz/
    Jenkinsfile
        imports ci/Jenkinsfile
        run Everything()
```

### Independent Jobs

1. run all tests
2. trigger all tests in other repositories using the current PR

```
ci/ [repository]
    linter/*
    Dockerfile
    Jenkinsfile
        def Everything():
            trigger core, satellite, storagenode
            wait responses

xyz/
    Jenkinsfile
        imports ci/Jenkinsfile
        run Everything()
```

## Minimal split

```
storj.io/core/encryption
storj.io/core/identity
storj.io/core/macaroon
storj.io/core/paths
storj.io/core/pb
storj.io/core/peertls
storj.io/core/peertls/extensions
storj.io/core/peertls/tlsopts
storj.io/core/pkcrypto
storj.io/core/ranger
storj.io/core/rpc
storj.io/core/rpc/rpcpeer
storj.io/core/rpc/rpcpool
storj.io/core/rpc/rpcstatus
storj.io/core/signing
storj.io/core/storj

storj.io/core/errs2
storj.io/core/fpath
storj.io/core/groupcancel
storj.io/core/memory
storj.io/core/readcloser
storj.io/core/sync2

storj.io/uplink                     <- storj.io/storj/uplink
storj.io/uplink/lib                 <- storj.io/storj/lib/uplink | aliases (maybe some other)
storj.io/uplink/ecclient            <- storj.io/storj/uplink/ecclient
storj.io/uplink/eestream            <- storj.io/storj/uplink/eestream
storj.io/uplink/metainfo            <- storj.io/storj/uplink/metainfo
storj.io/uplink/metainfo/kvmetainfo <- storj.io/storj/uplink/metainfo/kvmetainfo
storj.io/uplink/piecestore          <- storj.io/storj/uplink/piecestore
storj.io/uplink/storage/meta        <- storj.io/storj/uplink/storage/meta
storj.io/uplink/storage/objects     <- storj.io/storj/uplink/storage/objects
storj.io/uplink/storage/segments    <- storj.io/storj/uplink/storage/segments
storj.io/uplink/storage/streams     <- storj.io/storj/uplink/storage/streams
storj.io/uplink/stream              <- storj.io/storj/uplink/stream

storj.io/storj ==> {storj.io/core, storj.io/uplink}

kill for `uplink`:
	storj.io/storj/storage
```