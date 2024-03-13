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
#        u1 - v1.97.0            v1.99.0
#       /                        /
# m0 - m1 -- m2[v1.98.0-rc] -- m3 -- main
#             \
#              c1 - c2 - c3 - c4 - c5 - c6 - c7
#                \         \         \
#                 v1.98.1   v1.98.2   v1.98.3

commit main m0
commit m0   m1  v1.97.0-rc
commit m1   m2
commit m2   m3
commit m3   u1  v1.99.0
commit m3   m4

commit m2   c1
commit c1   c2
commit c2   c3
commit c3   c4
commit c4   c5
commit c5   c6
commit c6   c7

commit c1   q1  v1.98.1
commit c3   q2  v1.98.2
commit c5   q3  v1.98.3

commit m1   w1
commit w1   w2  v1.97.0

git checkout -q main
git reset --hard m4

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

check "main" v1.99.0

check m3      v1.98.3
check c7      v1.98.3
check c6      v1.98.3

check v1.98.3 v1.98.2
check c5      v1.98.2
check c4      v1.98.2

check v1.98.2 v1.98.1
check c3      v1.98.1
check c2      v1.98.1

check m2      v1.97.0

check_major v1.99.0    v1.98.3
check_major v1.98.3    v1.97.0
check_major v1.98.2    v1.97.0
check_major v1.98.1    v1.97.0

check m1 "did not find appropriate previous release tag"

echo "SUCCESS"
