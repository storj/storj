module storj.io/storj

go 1.13

replace google.golang.org/grpc => github.com/storj/grpc-go v1.27.2-0.20200225082019-bd19b647a81c

require (
	cloud.google.com/go v0.52.0
	github.com/BurntSushi/toml v0.3.1
	github.com/Shopify/go-lua v0.0.0-20181106184032-48449c60c0a9
	github.com/alessio/shellescape v0.0.0-20190409004728-b115ca0f9053
	github.com/alicebob/miniredis/v2 v2.11.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/boltdb/bolt v1.3.1
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/cheggaaa/pb/v3 v3.0.1
	github.com/fatih/color v1.7.0
	github.com/go-redis/redis v6.14.1+incompatible
	github.com/gogo/protobuf v1.2.1
	github.com/golang-migrate/migrate/v4 v4.7.0
	github.com/golang/protobuf v1.3.2
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/google/go-cmp v0.4.0
	github.com/gorilla/mux v1.7.1
	github.com/gorilla/schema v1.1.0
	github.com/graphql-go/graphql v0.7.9
	github.com/jackc/pgx v3.2.0+incompatible
	github.com/jtolds/go-luar v0.0.0-20170419063437-0786921db8c0
	github.com/jtolds/monkit-hw/v2 v2.0.0-20191108235325-141a0da276b3
	github.com/jtolds/tracetagger/v2 v2.0.0-rc3
	github.com/lib/pq v1.3.0
	github.com/loov/hrtime v0.0.0-20181214195526-37a208e8344e
	github.com/loov/plot v0.0.0-20180510142208-e59891ae1271
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/mattn/go-sqlite3 v2.0.2+incompatible
	github.com/nsf/jsondiff v0.0.0-20160203110537-7de28ed2b6e3
	github.com/nsf/termbox-go v0.0.0-20190121233118-02980233997d
	github.com/shopspring/decimal v0.0.0-20200105231215-408a2507e114
	github.com/skyrings/skyring-common v0.0.0-20160929130248-d1c0bb1cbd5e
	github.com/spacemonkeygo/monkit/v3 v3.0.1
	github.com/spf13/cast v1.3.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.4.0
	github.com/stripe/stripe-go v63.1.1+incompatible
	github.com/vivint/infectious v0.0.0-20190108171102-2455b059135b
	github.com/zeebo/admission/v2 v2.0.0
	github.com/zeebo/errs v1.2.2
	github.com/zeebo/structs v1.0.2
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200113162924-86b910548bc1
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	google.golang.org/grpc v1.26.0
	gopkg.in/yaml.v2 v2.2.4
	storj.io/common v0.0.0-20200226144507-3fe9f7839df5
	storj.io/drpc v0.0.8
	storj.io/uplink v1.0.0-rc.2
)
