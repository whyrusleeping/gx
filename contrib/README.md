# Scripts that make using GX nicer

## gx-retrotag

`gx-retrotag` retroactively adds git tags to commits that modify .gx/lastpubver
(gx release commits).

1. Fetches $remote (defaults to origin).
2. Tags the relevant commits (on $branch only, defaults to master).
3. Pushes the tags back to $remote.

Usage:

```sh
> ./gx-retrotag.sh [[remote] branch]
```
