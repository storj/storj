#!/usr/bin/env bash
set -uo pipefail

verlte() {
    # sort and check against the first result
    [  "$1" = "`echo -e "$1\n$2" | sort -V | head -n1`" ]
}
verlt() {
    [ "$1" = "$2" ] && return 1 || verlte $1 $2
}

# This script finds the previous tag from the current HEAD position.

closest_head_version="$(git describe --tags)"

sorted_tags="$(git tag --list --sort -version:refname)"

# this is ugly, but we use this approach to detect whether we are
# on a main branch, if that's the case, then we'll output the
# latest release version.
if [[ $closest_head_version =~ ^v1\.91\.0\-alpha ]]; then
    for tag in $sorted_tags
    do
        if [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo $tag
            exit 0
        fi
    done

    echo "did not find an appropriate release tag from main branch"
    exit 1
fi

IFS=$'\n'
for tag in $sorted_tags
do
    if [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        # the first less than our closest head should be our latest version
        if verlt $tag $closest_head_version; then
            echo $tag
            exit 0
        fi
    fi
done

echo "did not find an appropriate release tag"
exit 1
