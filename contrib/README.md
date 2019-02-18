# Scripts that make using GX nicer

## gx-retrotag

`gx-retrotag` retroactively adds git tags to commits that modify .gx/lastpubver
(gx release commits).

1. Fetches $remote (defaults to origin).
2. Tags the relevant commits (on $branch only, defaults to master).
3. Pushes the tags back to $remote.

**Usage:**

```sh
> ./gx-retrotag.sh [[remote] branch]
```

## gx-changelog

`gx-changelog` generates a recursive changelog of PRs for a release. Currently,
it only works with `go` projects hosted on GitHub that use a PR workflow.

**Usage:**

First, customize `REPO_FILTER` and `IGNORED_FILES` for your project:

- `REPO_FILTER` -- Selects the repos to be included in the changelog.
- `IGNORED_FILES` -- Specifies a set of files that should be excluded from the
  changelog. Any PRs _only_ touching these files will be ignored.

Then, run the script in your project's root repo.

**Requirements:**

This script requires:

* gx (obviously)
* jq
* go
* zsh (the _best_ shell)
* util-linux -- for the _awesome_ column command.
* git
* grep
* sed

If you care about that "portability", feel free to take a crack at improving
this script.
