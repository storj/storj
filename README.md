# Storj

[![Go Report Card](https://goreportcard.com/badge/github.com/storj/storj)](https://goreportcard.com/report/github.com/storj/storj)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/storj/storj)
<!-- [![Release](https://img.shields.io/github/release/golang-standards/project-layout.svg?style=flat-square)](https://github.com/storj/storj/releases/latest) -->
[![Coverage Status](https://coveralls.io/repos/github/storj/storj/badge.svg?branch=master)](https://coveralls.io/github/storj/storj?branch=master)

<img src="https://github.com/Storj/storj/blob/wip/logo/logo.png" width="100">

----

Storj is a platform, token, and suite of decentralized applications that allows you to store data in a secure and decentralized manner. Your files are encrypted, shredded into little pieces called 'shards' and stored in a global decentralized network of computers. Only you have access and the ability to retrieve all shards from the network, decrypt them, and finally re-combine all file pieces into your original file.

----

## To start using Storj

See our documentation at [Storj docs](https://docs.storj.io/docs).


## To start developing Storj

The [community site](https://storj.io/community.html) hosts all information about building storj from source, how to contribute code
and documentation, who to contact about what, etc.

### Install required packages

First of all install `git`, `golang` and `redis-server` and add environment variables.

#### Debian based (like Ubuntu)

```bash
apt-get install git golang redis-server
echo 'export GOPATH="$HOME/go"' >> $HOME/.bashrc
echo 'export PATH="$PATH:${GOPATH//://bin:}/bin"' >> $HOME/.bashrc
source $HOME/.bashrc
```

#### Mac OSX

```bash
brew install git go redis
if test -e $HOME/.bash_profile
then
	echo 'export GOPATH="$HOME/go"' >> $HOME/.bash_profile
	echo 'export PATH="$PATH:${GOPATH//://bin:}/bin"' >> $HOME/.bash_profile
	source $HOME/.bash_profile
else
	echo 'export GOPATH="$HOME/go"' >> $HOME/.profile
	echo 'export PATH="$PATH:${GOPATH//://bin:}/bin"' >> $HOME/.profile
	source $HOME/.profile
fi
```

### Install storj

Clone the storj repository. You may want to clone your own fork and branch.

```bash
git clone https://github.com/storj/storj $GOPATH/src/storj.io/storj
```

#### Install all dependencies

`go get` can be used to install all dependencies. The execution will take some time. You can add -v if you want to get more feedback.

```bash
go get -t storj.io/storj/...
```

Fix error message `cannot use "github.com/minio/cli"` See https://github.com/minio/minio/issues/5974 for more details.

```bash
go get -t github.com/minio/cli && rm -rf $GOPATH/src/github.com/minio/minio/vendor/github.com/minio/cli
go get -t storj.io/storj/...
```

### Run unit tests

```bash
go clean -testcache
go test storj.io/storj/...
```

You can execute only a single test package. For example: `go test storj.io/storj/pkg/kademlia`. Add -v for more informations about the executed unit tests.

### Start farmer

```bash
piecestore-farmer --help
```

To be continued.

## You have a working [Docker environment](https://docs.docker.com/engine).

```
$ git clone https://github.com/storj/storj
$ cd storj
$ make docker
```

For the full story, head over to the [developer's documentation].

## Support

If you need support, start with the [troubleshooting guide], and work your way through the process that we've outlined.


That said, if you have any questions or suggestions please reach out to us on [rocketchat](https://storj.io/community.html) or [twitter](https://twitter.com/storjproject).
