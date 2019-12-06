module storj.io/gateway

go 1.13

replace storj.io/storj => ../storj

require (
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/minio/cli v1.3.0
	github.com/minio/minio v0.0.0-20180508161510-54cd29b51c38
	github.com/spf13/cobra v0.0.5
	github.com/zeebo/errs v1.2.2
	go.uber.org/zap v1.10.0
	storj.io/storj v0.0.0-00010101000000-000000000000
)
