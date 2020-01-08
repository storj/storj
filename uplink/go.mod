module storj.io/uplink

go 1.13

exclude gopkg.in/olivere/elastic.v5 v5.0.72 // buggy import, see https://github.com/olivere/elastic/pull/869

replace google.golang.org/grpc => github.com/storj/grpc-go v1.23.1-0.20190918084400-1c4561bf5127

require (
	github.com/gogo/protobuf v1.2.1
	github.com/skyrings/skyring-common v0.0.0-20160929130248-d1c0bb1cbd5e
	github.com/spacemonkeygo/errors v0.0.0-20171212215202-9064522e9fd1 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/vivint/infectious v0.0.0-20190108171102-2455b059135b
	github.com/zeebo/errs v1.2.2
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200107144601-ef85f5a75ddf // indirect
	gopkg.in/spacemonkeygo/monkit.v2 v2.0.0-20190612171030-cf5a9e6f8fd2
	storj.io/common v0.0.0-20200108114547-1c62e5708bce
)
