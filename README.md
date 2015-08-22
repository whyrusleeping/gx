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

## Installation
To install (for now) you need to git clone it down, and then run `make` or
`make install` to install the binary to your `$GOPATH`

## Usage

Creating and publishing new generic package:

```bash
$ gx init
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

This downloads the package specified by the hash into the `vendor` directory in your
workspace. It also adds an entry referencing the package to the local `package.json`.

## ipfs

gx requires that you be running an ipfs daemon locally. If you run the daemon
on a port other than the default 5001, you can tell gx about it by setting the
`GX_IPFS_ADDR` environment variable to the address youre using, for example:

```
export GX_IPFS_ADDR=localhost:7777
```

## The vendor directory

The `vendor` (package) directory contains all of the downloaded dependencies of your
package.  You do not need to add the contents of the `vendor` directory to version
control, simply running `gx install` in the root directory of your project will
fetch and download the appropriate versions of required packages. 

Note: This is not to say that you can't add the `vendor` directory to version control,
by all means do if you want a single `git clone` or `svn co` to bring all deps
with it!

## Using gx as a Go package manager

If you want (like me) to use gx as a package manager for go, its pretty easy.
Pre go1.5, youll need to set your `GOPATH` to `$GOPATH:$(pwd)/vendor` and 
running `go build` or `go install`. Once go1.5 lands, you'll be able to build by
simply running `go build -vendor` or `go install -vendor`.

To import code from the vendor directory use:

```go
import "<hash>/packagename"
```

for example:
```go
import "QmR5FHS9TpLbL9oYY8ZDR3A7UWcHTBawU1FJ6pu9SvTcPa/cobra"
```

## TODO:
- in place package updating
- registries for naming
