#!/bin/bash

remote=${1:-origin}
branch=${2:-master}

echo "fetching $remote"
git fetch "$remote"

# Register the created tags to push
tags=""

for h in $(git log "$remote/$branch" --format=format:'%H' .gx/lastpubver); do
    # get the gx version at this point
    ver="$(git show $h:.gx/lastpubver 2>/dev/null | cut -d: -f1)" || continue

    # Skip empty versions
    [[ -n "$ver" ]] || continue


    # skip if the tag exists
    if git show-ref "v$ver" "$ver" >/dev/null; then
        continue
    fi

    # tag it.
    echo "tagging $ver ($h)"
    git tag -s -m "release $ver" "v$ver" $h
    tags="$tags tag v$ver"
done

if [[ ! -z "$tags" ]]; then
    echo "pushing tags to $remote"
    git push "$remote" $tags
else
    echo "nothing to do"
fi
