# Uplink

Documentation for developing and building the uplink service.

Usage:

First make an identity:
```
git clone https://github.com/storj/storj
cd storj
go install storj.io/storj/cmd/identity
identity create uplink
```

Then setup the uplink:
```
go install storj.io/storj/cmd/uplink
uplink setup
```

You can edit `~/.storj/uplink/config.yaml` to your liking. Then run it!

```
uplink run
```
