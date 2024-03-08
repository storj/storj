#!/usr/bin/env bash
set -ueo pipefail

# This script finds the previous tag from the current HEAD position.

closestHeadVersion="$(git describe --tags)"

sortedTags="$(git tag --list --sort -version:refname)"

verlte() {
    # sort and get the first result
    [  "$1" = "`echo -e "$1\n$2" | sort -V | head -n1`" ]
}
verlt() {
    [ "$1" = "$2" ] && return 1 || verlte $1 $2
}

IFS=$'\n'
for tag in $sortedTags
do
    if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        # it's not a proper version so ignore
        continue
    fi

    # the first less than our closest head should be our latest version
    if verlt $tag $closestHeadVersion; then
        echo $tag
        exit 0
    fi
done

echo "did not find an appropriate release tag"
exit 1
