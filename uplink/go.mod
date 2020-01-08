module storj.io/uplink

go 1.13

replace storj.io/storj => ../

exclude gopkg.in/olivere/elastic.v5 v5.0.72 // buggy import, see https://github.com/olivere/elastic/pull/869

replace google.golang.org/grpc => github.com/storj/grpc-go v1.23.1-0.20190918084400-1c4561bf5127

require (
	github.com/gogo/protobuf v1.2.1
	github.com/skyrings/skyring-common v0.0.0-20160929130248-d1c0bb1cbd5e
	github.com/stretchr/testify v1.3.0
	github.com/vivint/infectious v0.0.0-20190108171102-2455b059135b
	github.com/zeebo/errs v1.2.2
	go.uber.org/zap v1.10.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	gopkg.in/spacemonkeygo/monkit.v2 v2.0.0-20190612171030-cf5a9e6f8fd2
	storj.io/common v0.0.0-20200107155525-ccc8474e4234
	storj.io/storj v0.12.1-0.20200108014024-c740b82e6675 // indirect
)
