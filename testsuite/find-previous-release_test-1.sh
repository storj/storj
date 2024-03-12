#!/usr/bin/env bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT INT

cd $TMPDIR

# Create the testing git repo.

commit() {
    from=$1
    to=$2
    tag=${3:-""}

    git checkout -q $from
    git checkout -b $to
    git commit --allow-empty -m"$from -> $to [$tag]"

    if [ "$tag" ]; then
        git tag $3
    fi
}

git -c init.defaultBranch=main -c user.name="User" -c user.email="user@test" init .
git config user.name "User"
git config user.email "user@test"
git config advice.detachedHead false
git commit --allow-empty -m"R"

# COMMIT GRAPH UNDER TEST
#
#                          _ v1.98.0-rc - v1.98.0 - m2c - v1.98.1 - m2e - v1.98.2
#                         /
# R - m0[v1.97.0] - m1 - m2 - m3 - m4 - m5 - m6 - m7 - m8[main]
#           \                  \              \
#            \                  - x            - v1.99.0-rc - v1.99.0 - v1.99.1
#             \                                       \
#              \- v1.97.1 - v1.97.2                    - y

commit main m0 v1.97.0

commit m0 m1
commit m1 m2
commit m2 m3
commit m3 m4
commit m4 m5
commit m5 m6
commit m6 m7
commit m7 m8

commit m0  m0a v1.97.1
commit m0a m0b v1.97.2

commit m2  m2a v1.98.0-rc
commit m2a m2b v1.98.0
commit m2b m2c
commit m2c m2d v1.98.1
commit m2d m2e
commit m2e m2f v1.98.2

commit m3 x

commit m6  m6a v1.99.0-rc
commit m6a m6b v1.99.0
commit m6b m6c v1.99.1

commit m6a y

git checkout -q main
git reset --hard m8

# Do the testing stuff.

check() {
    echo "checking at '$1' expecting '$2'"

    git checkout -q $1
    got=$($SCRIPTDIR/find-previous-release.sh || true)

    if [[ $got != $2 ]];
    then
        echo "  FAILED got '$got'"
        exit 1
    fi
}

check_major() {
    echo "checking -major at '$1' expecting '$2'"

    git checkout -q $1
    got=$($SCRIPTDIR/find-previous-release.sh --major || true)

    if [[ $got != $2 ]];
    then
        echo "  FAILED got '$got'"
        exit 1
    fi
}

check main v1.99.1
check m7   v1.99.1

check v1.99.1    v1.99.0
check v1.99.0-rc v1.98.2
check v1.99.0    v1.98.2
check y          v1.99.1

check m6 v1.98.2
check m5 v1.98.2
check m4 v1.98.2
check m3 v1.98.2
check x  v1.98.2

check v1.98.2    v1.98.1
check m2e        v1.98.1
check v1.98.1    v1.98.0
check m2c        v1.98.0
check v1.98.0    v1.97.2
check v1.98.0-rc v1.97.2

check m2 v1.97.2
check m1 v1.97.2

check v1.97.2 v1.97.1
check v1.97.1 v1.97.0

check_major v1.99.1    v1.98.2
check_major v1.99.0-rc v1.98.2
check_major v1.99.0    v1.98.2

check_major v1.98.2    v1.97.2
check_major v1.98.1    v1.97.2
check_major v1.98.0    v1.97.2

check v1.97.0 "did not find appropriate previous release tag"
check m0      "did not find appropriate previous release tag"

check_major v1.97.0    "did not find appropriate previous release tag"

echo "SUCCESS"
