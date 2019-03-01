#!/bin/zsh

[[ -n "$REPO_FILTER" ]] || REPO_FILTER="github.com/(ipfs|libp2p|ipld)"
[[ -n "$IGNORED_FILES" ]] || IGNORED_FILES='^\(\.gx\|package.json\|\.travis.yml\)$'

export GOPATH="$(go env GOPATH)"

alias jq="jq --unbuffered"

set -e
set -o pipefail

# Returns a json object recursively listing all deps of the current package.
gx_deps() {
    gx deps -r | column  -J --table-columns name,hash,version | jq '.table'
}

# Convert a stream into an array.
slurp() {
    jq -s .
}

# Convert an array into a stream.
stream() {
    jq '.[]'
}

# Returns a stream of deps changed between $1 and $2.
dep_changes() {
    {
        <"$1"
        <"$2"
    } | jq -s 'JOIN(INDEX(.[0][]; .repo); .[1][]; .repo; {repo: .[0].repo, old: .[1].hash, new: .[0].hash}) | select(.new != .old)'
}

# Replace all gx package names with go import paths.
resolve_repos() {
    jq -r '"\(env.GOPATH)/src/gx/ipfs/\(.hash)/\(.name)/package.json \(@json)"' |
        while read gxfile json; do
            jq --argjson pkg "$json" '($pkg|del(.name)) + {repo: .gx.dvcsimport}' "$gxfile"
        done
}

# Takes in a stream of objects in the form:
# `{ repo: path, old: GxHash, new: GxHash }`
# And returns a stream of objects in the form:
# `{ repo: phat, old: CommitHash, new: CommitHash }`
resolve_commits() {
    local repo new old
    jq -r '"\(.repo) \(.new) \(.old)"' | while read repo new old; do
        local old_commit="null"
        local new_commit="null"
        local dir="$GOPATH/src/$repo"
        while read commit; do
            hash="$(git -C "$dir" show "$commit:.gx/lastpubver" 2>/dev/null | cut -d' ' -f2)"
            case "$hash" in
                "$new") new_commit="$(printf '"%q"' "$commit")" ;;
                "$old") old_commit="$(printf '"%q"' "$commit")" ;;
            esac
        done < <(git -C "$dir" log origin/master --format=tformat:'%H' .gx/lastpubver)
        printf '{"repo": "%q", "old": %s, "new": %s}\n' "$repo" "$old_commit" "$new_commit"
    done | jq
}

# Generate a release log for a range of commits in a single repo.
release_log() {
    local repo="$1"
    local start="$2"
    local end="${3:-HEAD}"
    local ghname="${repo##github.com/}"
    local dir="$GOPATH/src/$repo"
    
        local commit prnum
        git -C "$dir" log \
            --format='tformat:%H %s' \
            --merges \
            "$start..$end"  |
            sed -n -e 's/\([a-f0-9]\+\) Merge pull request #\([0-9]\+\) from .*/\1 \2/p' |
            while read commit prnum; do
                # Skip gx-only PRs.
                git -C "$dir" diff-tree --no-commit-id --name-only "$commit^" "$commit" |
                        grep -v "${IGNORED_FILES}" >/dev/null || continue

                local desc="$(git -C "$dir" show --summary --format='tformat:%b' "$commit" | head -1)"
                printf "- %s ([%s#%s](https://%s/pull/%s))\n" "$desc" "$ghname" "$prnum" "$repo" "$prnum"
            done
}

indent() {
    sed -e 's/^/  /'
}

recursive_release_log() {
    local start="${1:-$(git tag -l | sort -V | grep -v -- '-rc' | grep 'v'| tail -n1)}"
    local end="${2:-$(git rev-parse HEAD)}"
    local repo_root="$(git rev-parse --show-toplevel)"
    local package="$(go list)"
    (
        local workspace="$(mktemp -d)"
        trap "$(printf 'rm -rf "%q"' "$workspace")" INT TERM EXIT
        cd "$workspace"

        echo "Computing old deps..." >&2
        git -C "$repo_root" show "$start:package.json" > "package.json"
        gx install --nofancy >/dev/null
        gx_deps | stream | resolve_repos | slurp > old_deps.json

        echo "Computing new deps..." >&2
        git -C "$repo_root" show "$end:package.json" > "package.json"
        gx install --nofancy >/dev/null
        gx_deps | stream | resolve_repos | slurp > new_deps.json

        rm package.json

        echo "Generating Changelog..." >&2

        printf "- %s:\n" "$package"
        release_log "$package" "$start" "$end" | indent

        dep_changes old_deps.json new_deps.json |
            # Filter by repo
            jq --arg filter "$REPO_FILTER" 'select(.repo | match($filter))' |
            # Add in commit ranges.
            resolve_commits |
            # Compute changelogs
            jq -r '"\(.repo) \(.new // "origin/master") \(.old // "")"' |
            while read repo new old; do
                local changelog="$(release_log "$repo" "$old" "$new")"
                if [[ -n "$changelog" ]]; then
                    printf "- %s:\n" "$repo"
                    echo "$changelog" | indent
                fi
            done
    )
}

recursive_release_log "$@"
