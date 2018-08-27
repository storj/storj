set -x

pushd $HOME

virtualenv my_py3 --python=/usr/bin/python3.4
source my_py3/bin/activate
pip install --upgrade awscli

popd

set +x
