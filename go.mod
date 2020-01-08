module storj.io/storj

go 1.13

// force specific versions for minio
require (
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/graphql-go/graphql v0.7.9-0.20190403165646-199d20bbfed7

	github.com/minio/minio v0.0.0-20180508161510-54cd29b51c38
	github.com/segmentio/go-prompt v1.2.1-0.20161017233205-f0d19b6901ad
)

exclude gopkg.in/olivere/elastic.v5 v5.0.72 // buggy import, see https://github.com/olivere/elastic/pull/869

replace google.golang.org/grpc => github.com/storj/grpc-go v1.23.1-0.20190918084400-1c4561bf5127

require (
	github.com/Shopify/go-lua v0.0.0-20181106184032-48449c60c0a9
	github.com/alessio/shellescape v0.0.0-20190409004728-b115ca0f9053
	github.com/alicebob/miniredis v2.5.0+incompatible
	github.com/blang/semver v3.5.1+incompatible
	github.com/boltdb/bolt v1.3.1
	github.com/cheggaaa/pb/v3 v3.0.1
	github.com/cockroachdb/cockroach-go v0.0.0-20181001143604-e0a95dfd547c
	github.com/fatih/color v1.7.0
	github.com/go-redis/redis v6.14.1+incompatible
	github.com/gogo/protobuf v1.2.1
	github.com/golang-migrate/migrate/v4 v4.7.0
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.0
	github.com/gorilla/mux v1.7.1
	github.com/gorilla/schema v1.1.0
	github.com/jtolds/go-luar v0.0.0-20170419063437-0786921db8c0
	github.com/jtolds/monkit-hw v0.0.0-20190108155550-0f753668cf20
	github.com/lib/pq v1.3.0
	github.com/loov/hrtime v0.0.0-20181214195526-37a208e8344e
	github.com/loov/plot v0.0.0-20180510142208-e59891ae1271
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/minio/cli v1.3.0
	github.com/minio/minio-go v6.0.3+incompatible
	github.com/nsf/jsondiff v0.0.0-20160203110537-7de28ed2b6e3
	github.com/nsf/termbox-go v0.0.0-20190121233118-02980233997d
	github.com/skyrings/skyring-common v0.0.0-20160929130248-d1c0bb1cbd5e
	github.com/spf13/cast v1.3.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.3.0
	github.com/stripe/stripe-go v63.1.1+incompatible
	github.com/vivint/infectious v0.0.0-20190108171102-2455b059135b
	github.com/zeebo/admission v0.0.0-20180821192747-f24f2a94a40c
	github.com/zeebo/errs v1.2.2
	github.com/zeebo/structs v1.0.2
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200107144601-ef85f5a75ddf
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	google.golang.org/grpc v1.23.1
	gopkg.in/spacemonkeygo/monkit.v2 v2.0.0-20190612171030-cf5a9e6f8fd2
	gopkg.in/yaml.v2 v2.2.2
	storj.io/common v0.0.0-20200107155525-ccc8474e4234
	storj.io/drpc v0.0.7-0.20191115031725-2171c57838d2
	storj.io/uplink v0.0.0-20200108122946-2f76cf51fe31
)
