module storj.io/redirect/gateway

// module uses different name to make go module system treat it as a separate
// package and make it such there isn't a cyclic dependency.

go 1.13

require storj.io/gateway v1.0.0-rc.2

exclude gopkg.in/olivere/elastic.v5 v5.0.72 // buggy import, see https://github.com/olivere/elastic/pull/869

replace google.golang.org/grpc => github.com/storj/grpc-go v1.27.2-0.20200225082019-bd19b647a81c
