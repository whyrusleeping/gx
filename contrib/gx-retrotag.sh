#!/bin/bash

remote=${1:-origin}
branch=${2:-master}

echo "fetching $remote"
git fetch "$remote"

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
    changed=true
done

if [[ -n "$changed" ]]; then
    echo "pushing tags to $remote"
    git push --tags --repo="$remote"
else
    echo "nothing to do"
fi
