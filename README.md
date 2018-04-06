# Storj

[![Go Report Card](https://goreportcard.com/badge/github.com/golang-standards/project-layout?style=flat-square)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
[![Release](https://img.shields.io/github/release/golang-standards/project-layout.svg?style=flat-square)](https://github.com/storj/storj/releases/latest)

<img src="https://github.com/Storj/storj/blob/wip/logo/logo.png" width="100">

----

Storj is a platform, cryptocurrency, and suite of decentralized applications that allows you to store data in a secure and decentralized manner. Your files are encrypted, shredded into little pieces called 'shards', and stored in a decentralized network of computers around the globe. No one but you has a complete copy of your file, not even in an ecrypted form.

----

## To start using Storj

See our documentation at [storj docs](https://docs.storj.io/docs).


## To start developing storj

The [community site](https://storj.io/community.html) hosts all information about
building storj from source, how to contribute code
and documentation, who to contact about what, etc.

If you want to build storj right away there are two options:

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

If you need support, start with the [troubleshooting guide],
and work your way through the process that we've outlined.

That said, if you have questions, reach out to us
[twitter](https://twitter.com/storjproject).




