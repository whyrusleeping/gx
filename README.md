# GX
## A packaging tool
### (gx means nothing)

gx is a packaging tool built around the distributed, content addressed filesystem
ipfs. It aims to be flexible, powerful and simple (after learning the commands, of
course).

## Requirements
gx currently requires that users have a running ipfs daemon on their machine.
This requirement may be lifted in the future when better infrastructure is set
up.

Creating and publishing new generic package:

```bash
$ gx init
$ gx add ./*
$ gx publish
```

This will output a 'package-hash' which is unique to the exact content of your
package at the time of publishing. If someone were to download your package and
republish it, it would produce the *exact* same hash.

To add a dependency of another package to your package, simply import it by its
hash:

```bash
$ gx import QmaDFJvcHAnxpnMwcEh6VStYN4v4PB4S16j4pAuC2KSHVr
```

This downloads the package specified by the hash into the `pkg` directory in your
workspace. It also adds an entry referencing the package to the local `package.json`.

## The pkg directory

The `pkg` (package) directory contains all of the downloaded dependencies of your
package.  You do not need to add the contents of the `pkg` directory to version
control, simply running `gx install` in the root directory of your project will
fetch and download the appropriate versions of required packages. 

Note: This is not to say that you can't add the `pkg` directory to version control,
by all means do if you want a single `git clone` or `svn co` to bring all deps
with it!

## TODO:
- in place package updating
- registries for naming
