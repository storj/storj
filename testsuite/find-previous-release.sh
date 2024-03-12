#!/usr/bin/env bash
set -ueo pipefail

find_major_release=
if [[ "${1:-}" == "--major" ||  "${1:-}" == "-major" ]]; then
    find_major_release=1
fi

debug=
minimum_checked_version="v1.50.0"

trace() {
    if [ $debug ]; then
        >&2 echo "$1"
    fi
}

verlte() {
    # sort and check against the first result
    [  "$1" = "`echo -e "$1\n$2" | sort -V | head -n1`" ]
}

find_release_on_strand() {
    local query=$2
    local furthest_ancestor=$3
    local furthest_ancestor_hash=$(git rev-list -n 1 $furthest_ancestor)
    local main=${4:-""}

    trace "  # find_release_on_strand $query $furthest_ancestor $main"

    local query_tag=$(git describe --tags --exact-match $query 2> /dev/null)
    local query_parent_tag=$(git describe --tags --exact-match "$query^" 2> /dev/null)

    trace "    # current $query_tag:$query_parent_tag"

    for tag in $(git tag --list --sort -version:refname)
    do
        # trace "    # checking out $tag"
        # Check whether it's a proper version, if it's not then skip.
        if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            # trace "    # skipping $tag because it is not a proper release"
            continue
        fi

        if verlte "$tag" "$minimum_checked_version"; then
            # trace "    # skipping $tag because it is a really old release"
            continue
        fi

        # We want to exclude tags that are not descendants of furthest_ancestor.
        if ! git merge-base --is-ancestor $furthest_ancestor $tag; then
            trace "    # skipping $tag because $query is furthest ancestor of $tag"
            continue
        fi

        # We want to exclude tags that are descendants of query.
        if git merge-base --is-ancestor $query $tag; then
            trace "    # skipping $tag because $query is parent of $tag"
            continue
        fi

        # We want to exclude tags that are descendants of query's parent, if
        # the query is tagged and the query's parent is not tagged. this is the
        # special case for scripts/tag-release.sh tagging behavior.

        if [ "$query_tag" ]; then
            if ! [ "$query_parent_tag" ]; then
                if git merge-base --is-ancestor "$query^" $tag; then
                    trace "    # skipping $tag because $query is tag-release special case"
                    continue
                fi
            fi
        fi

        if [ "$main" ]; then
            # I we are not yet evaluating the main branch, we want to exclude tags
            # that branched off the main branch later than we did.
            tag_branching_point=$(git merge-base $main $tag)
            if [ $tag_branching_point != $furthest_ancestor_hash ]; then
                if git merge-base --is-ancestor $furthest_ancestor $tag_branching_point; then
                    trace "    # skipping $tag because it branched off from main later than $query"
                    continue
                fi
            fi
        fi

        eval "$1=\$tag"
        return 0
    done
}

main="main"
if ! git rev-parse --verify $main; then
    main=remotes/origin/main
fi

query="HEAD"
if [[ $find_major_release ]]; then
    query=$(git merge-base $main HEAD)
fi

release=""
find_release_on_strand release $query $(git merge-base $main $query) $main
if [ "$release" ]; then
    echo "$release"
    exit 0
fi

root=$(git rev-list --max-parents=0 $query)
find_release_on_strand release $(git merge-base $main $query) $root ""
if [ "$release" ]; then
    echo "$release"
    exit 0
fi

echo "did not find appropriate previous release tag"
exit 1