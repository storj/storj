set -x

mkdir -p $GOPATH/src/storj.io
mv $GOPATH/src/github.com/storj/storj $GOPATH/src/storj.io
export TRAVIS_BUILD_DIR=$GOPATH/src/storj.io/storj
cd $TRAVIS_BUILD_DIR

pushd $HOME

virtualenv my_py3 --python=/usr/bin/python3.4
source my_py3/bin/activate
pip install --upgrade awscli

popd

set +x
