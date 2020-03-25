module storj.io/storj/cmd/gateway

// module uses different name to make go module system treat it as a separate
// package and make it such there isn't a cyclic dependency.

go 1.13

require storj.io/gateway v1.0.0-rc.6.0.20200320070749-1255a4ca40d9

require (
	github.com/howeyc/gopass v0.0.0-20170109162249-bf9dde6d0d2c // indirect
	github.com/segmentio/go-prompt v1.2.1-0.20161017233205-f0d19b6901ad // indirect
	// keep this in sync with storj.io/gateway
	storj.io/storj v0.12.1-0.20200320013728-9b78473c0c76
)

exclude gopkg.in/olivere/elastic.v5 v5.0.72 // buggy import, see https://github.com/olivere/elastic/pull/869

replace google.golang.org/grpc => github.com/storj/grpc-go v1.27.2-0.20200225082019-bd19b647a81c
