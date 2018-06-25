# Admin Node

To start the admin node:

```
go run cmd/admin/main.go -redisAddress localhost:6379 -srvPort 8081
```

You need to set `redisAdress` to point to your local Redis instance, and `-srvPort` as something other than `8080`

You should see output similar to this when everything is up and running:

```
> $ go run cmd/admin/main.go -redisAddress localhost:6379 -srvPort 8081                                                                                                                 ⬡ 8.9.3 [±admin ●●]
bootstrapping cache
unreachable: [214 170 31 194 250 116 19 142 148 84 87 81 130 188 112 183 225 69 55 214]
[]
unreachable: [214 170 31 194 250 116 19 142 148 84 87 81 130 188 112 183 225 69 55 214]
unreachable: [214 170 31 194 250 116 19 142 148 84 87 81 130 188 112 183 225 69 55 214]```
