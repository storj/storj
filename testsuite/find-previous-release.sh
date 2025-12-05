#!/usr/bin/env bash
set -ue

find_major_release=
if [[ "${1:-}" == "--major" ||  "${1:-}" == "-major" ]]; then
    find_major_release=1
fi

debug=
minimum_checked_version="v1.50.0"

trace() {
    if [ "$debug" ]; then
        >&2 echo "$1"
    fi
}

verlte() {
    # sort and check against the first result
    [  "$1" = "$(echo -e "$1\n$2" | sort -V | head -n1)" ]
}

find_release_on_strand() {
    local query
    local main
    local furthest_ancestor
    local query_tag
    local query_parent_tag

    query=$2
    furthest_ancestor=$3
    main=${4:-""}

    trace "  # find_release_on_strand $query $furthest_ancestor $main"

    query_tag=$(git describe --tags --exact-match "$query" 2> /dev/null || true)
    query_parent_tag=$(git describe --tags --exact-match "$query^" 2> /dev/null || true)

    trace "    # current $query_tag:$query_parent_tag"

    local query_branching_point
    query_branching_point=$(git merge-base "$main" "$query")

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
        if git merge-base --is-ancestor "$query" "$tag"; then
            trace "    # skipping $tag because $query is parent of $tag"
            continue
        fi

        # We want to exclude tags that are descendants of query's parent, if
        # the query is tagged and the query's parent is not tagged. this is the
        # special case for scripts/tag-release.sh tagging behavior.

        if [ "$query_tag" ]; then
            if ! [ "$query_parent_tag" ]; then
                if git merge-base --is-ancestor "$query^" "$tag"; then
                    trace "    # skipping $tag because $query is tag-release special case"
                    continue
                fi
            fi
        fi

        local tag_branching_point
        tag_branching_point=$(git merge-base "$main" "$tag")
        if [ "$tag_branching_point" != "$query_branching_point" ]; then
            if git merge-base --is-ancestor "$query_branching_point" "$tag_branching_point"; then
                trace "    # skipping $tag because it branched off from main later than $query"
                continue
            fi
        fi

        eval "$1=\$tag"
        return 0
    done
}

main="main"
if ! git rev-parse -q --verify "$main" 1>/dev/null; then
    main=remotes/origin/main
fi

query="HEAD"
if [[ "$find_major_release" ]]; then
    query=$(git merge-base "$main" HEAD)
fi

branching_point=$(git merge-base "$main" "$query")

release=""
find_release_on_strand release "$query" "$branching_point" "$main"
if [ "$release" ]; then
    echo "$release"
    exit 0
fi

root=$(git rev-list --max-parents=0 $query)
find_release_on_strand release "$branching_point" "$root" "$main"
if [ "$release" ]; then
    echo "$release"
    exit 0
fi

echo "did not find appropriate previous release tag"
exit 1