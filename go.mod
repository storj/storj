module storj.io/storj

go 1.13

require (
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/garyburd/redigo v1.0.1-0.20170216214944-0d253a66e6e1 // indirect
	github.com/graphql-go/graphql v0.7.9-0.20190403165646-199d20bbfed7

	github.com/minio/minio v0.0.0-20180508161510-54cd29b51c38
	github.com/segmentio/go-prompt v1.2.1-0.20161017233205-f0d19b6901ad
)

exclude gopkg.in/olivere/elastic.v5 v5.0.72 // buggy import, see https://github.com/olivere/elastic/pull/869

replace google.golang.org/grpc => github.com/storj/grpc-go v1.23.1-0.20190918084400-1c4561bf5127

require (
	github.com/Shopify/go-lua v0.0.0-20181106184032-48449c60c0a9
	github.com/alessio/shellescape v0.0.0-20190409004728-b115ca0f9053
	github.com/alicebob/gopher-json v0.0.0-20180125190556-5a6b3ba71ee6 // indirect
	github.com/alicebob/miniredis v0.0.0-20180911162847-3657542c8629
	github.com/armon/go-metrics v0.0.0-20180917152333-f0300d1749da // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/boltdb/bolt v1.3.1
	github.com/cheggaaa/pb v1.0.5-0.20160713104425-73ae1d68fe0b // indirect
	github.com/cheggaaa/pb/v3 v3.0.1
	github.com/djherbis/atime v1.0.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/eclipse/paho.mqtt.golang v1.1.1 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.0 // indirect
	github.com/fatih/color v1.7.0
	github.com/fatih/structs v1.0.0 // indirect
	github.com/go-ini/ini v1.38.2 // indirect
	github.com/go-redis/redis v6.14.1+incompatible
	github.com/gogo/protobuf v1.2.1
	github.com/golang-migrate/migrate/v4 v4.7.0
	github.com/google/go-cmp v0.5.2
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/graphql-go/graphql v0.7.9
	github.com/jackc/pgconn v1.7.0
	github.com/jackc/pgtype v1.5.0
	github.com/jackc/pgx/v4 v4.9.0
	github.com/jtolds/monkit-hw/v2 v2.0.0-20191108235325-141a0da276b3
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/nsf/jsondiff v0.0.0-20200515183724-f29ed568f4ce
	github.com/nsf/termbox-go v0.0.0-20200418040025-38ba6e5628f1
	github.com/shopspring/decimal v1.2.0
	github.com/spacemonkeygo/monkit/v3 v3.0.7-0.20200515175308-072401d8c752
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/stripe/stripe-go v70.15.0+incompatible
	github.com/vivint/infectious v0.0.0-20200605153912-25a574ae18a3
	github.com/zeebo/errs v1.2.2
	github.com/zeebo/float16 v0.1.0 // indirect
	github.com/zeebo/incenc v0.0.0-20180505221441-0d92902eec54 // indirect
	github.com/zeebo/structs v1.0.2
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7
	golang.org/x/net v0.0.0-20190916140828-c8589233b77d // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20190916202348-b4ddaad3f8a3
	golang.org/x/tools v0.0.0-20190917215024-905c8ffbfa41
	google.golang.org/appengine v1.6.0 // indirect
	google.golang.org/genproto v0.0.0-20190716160619-c506a9f90610 // indirect
	google.golang.org/grpc v1.23.1
	gopkg.in/Shopify/sarama.v1 v1.18.0 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.25 // indirect
	gopkg.in/ini.v1 v1.38.2 // indirect
	gopkg.in/olivere/elastic.v5 v5.0.76 // indirect
	gopkg.in/spacemonkeygo/monkit.v2 v2.0.0-20190612171030-cf5a9e6f8fd2
	gopkg.in/yaml.v2 v2.2.2
	storj.io/drpc v0.0.7-0.20191115031725-2171c57838d2
)
