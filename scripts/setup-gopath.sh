set -x

mkdir -p ~/bin ~/cache
export PATH=~/bin:$PATH

export GOSPACE_ROOT=$GOPATH
export GOSPACE_PKG=storj.io/storj
export GOSPACE_REPO=git@github.com:storj/storj/git

# setup gospace
wget -O ~/bin/gospace https://github.com/storj/gospace/releases/download/v0.0.5/gospace_linux_amd64
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

set +x
