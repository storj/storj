# Storj

[![Go Report Card](https://goreportcard.com/badge/github.com/golang-standards/project-layout?style=flat-square)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Release](https://img.shields.io/github/release/golang-standards/project-layout.svg?style=flat-square)](https://github.com/storj/storj/releases/latest)

<img src="https://github.com/Storj/storj/blob/wip/logo/logo.png" width="100">

----

Storj is a platform, token, and suite of decentralized applications that allows you to store data in a secure and decentralized manner. Your files are encrypted, shredded into little pieces called 'shards' and stored in a global decentralized network of computers. Only you have access and the ability to retrieve all shards from the network, decrypt them, and finally re-combine all file pieces into your original file.

----

## To start using Storj

See our documentation at [Storj docs](https://docs.storj.io/docs).


## To start developing Storj

The [community site](https://storj.io/community.html) hosts all information about building storj from source, how to contribute code
and documentation, who to contact about what, etc.

### Install VGO

```Go
go get -u golang.org/x/vgo
```

### Install non-go development dependencies

In order to develop on Storj, you will need to have the `protobuf` compiler installed on your system.

1. Grab the latest release for your system from [here](https://github.com/google/protobuf/releases).

1. place the `protoc` binary in your path. i.e.

    ```bash
    mv $HOME/Downloads/protoc-<version>-<arch>/bin/protoc /usr/local/bin
    ```

### Install go dependencies

Use vgo to install both dev and non-dev dependencies.

1. Install development dependencies

    ```
    make build-dev-deps
    ```

1. Install project dependencies

    ```bash
    # in project root
    vgo install
    ```


If you want to build Storj right away there are two options:

##### You have a working [Go environment](https://golang.org/doc/install).

```
$ go get -d storj.io/storj
$ cd $GOPATH/src/storj.io/storj
$ make
```

##### You have a working [Docker environment](https://docs.docker.com/engine).

```
$ git clone https://github.com/storj/storj
$ cd storj
$ make docker
```

For the full story, head over to the [developer's documentation].

## Support

If you need support, start with the [troubleshooting guide], and work your way through the process that we've outlined.


That said, if you have any questions or suggestions please reach out to us on [rocketchat](https://storj.io/community.html) or [twitter](https://twitter.com/storjproject).
