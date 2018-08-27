set -x

git clone git@github.com:storj/storj.git storj
export TRAVIS_BUILD_DIR=$HOME/storj
cd $TRAVIS_BUILD_DIR

virtualenv my_py3 --python=/usr/bin/python3.4
source my_py3/bin/activate
pip install --upgrade awscli

set +x
