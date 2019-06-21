# Link Sharing Service

## Building

```
$ go install storj.io/storj/cmd/linksharing
```

## Configuring

### Development

Default development configuration has the link sharing service hosted on
`localhost:8080` serving plain HTTP.

```
$ linksharing setup --defaults dev
```

### Production (HTTPS)

Default production configuration has the link sharing service hosted on `:8443`
serving HTTPS using a server certificate (`server.crt.pem`) and
key (`server.key.pem`) residing in the working directory where the linksharing
service is run.

```
$ linksharing setup --defaults release
```

You can modify the configuration file or use the `--cert-file` and `--key-file`
flags to configure an alternate location for the server keypair.

### Production (HTTP)

In order to run the link sharing service in release mode serving HTTP, you must
clear the certificate and key file configurables.

**WARNING** This is only recommended if you are doing TLS termination on the
same machine running the link sharing service as the link sharing service
serves unencrypted user data.

```
$ linksharing setup --defaults release --cert-file="" --key-file="" --address=":8080"
```

## Running

After configuration is complete, running the link sharing is as simple as:

```
$ linksharing run
```
