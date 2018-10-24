# Gateway

Documentation for developing and building the gateway service

Usage:

First make an identity:
```
go install storj.io/storj/cmd/gateway
gateway setup
```

The gateway shares the uplink config file.
You can edit `~/.storj/uplink/config.yaml` to your liking. Then run it!

```
gateway run
```
