set -x

mkdir -p ~/bin ~/cache
export PATH=~/bin:$PATH

export GOSPACE_ROOT=$GOPATH
export GOSPACE_PKG=storj.io/storj
export GOSPACE_REPO=git@github.com:storj/storj/git

mkdir -p $GOPATH/src/storj.io
mv $GOPATH/src/github.com/storj/storj $GOPATH/src/storj.io

# TODO: setup gospace
mv $GOPATH/src/storj.io/storj/scripts/gospace ~/bin
chmod +x ~/bin/gospace

# find module dependency hash
MODHASH=$(gospace hash)

# download dependencies, if we don't have them in cache
if [ ! -f $HOME/cache/$MODHASH.zip ]; then
    gospace zip-vendor $HOME/cache/$MODHASH.zip
fi

# unpack the dependencies into gopath
gospace unzip-vendor $HOME/cache/$MODHASH.zip
gospace flatten-vendor

export TRAVIS_BUILD_DIR=$GOPATH/src/storj.io/storj
cd $TRAVIS_BUILD_DIR

set +x
