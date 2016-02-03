![gx logo](logo.jpeg)

# GX
> A packaging tool

gx is a packaging tool built around the distributed, content addressed filesystem
[IPFS](//github.com/ipfs/ipfs). It aims to be flexible, powerful and simple.

## Table of Contents
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Dependencies](#dependencies)
- [Updating](#updating)
- [Repos](#repos)
  - [Usage](#usage-1)
- [Hooks](#hooks)
- [The vendor directory](#the-vendor-directory)
- [Using gx as a Go package manager](#using-gx-as-a-go-package-manager)
- [Using gx as a package manager for language/environment X](#using-gx-as-a-package-manager-for-languageenvironment-x)
- [Why is it called gx?](#why-is-it-called-gx)
- [License](#license)

## Requirements
gx currently requires that users have a running [IPFS daemon](//github.com/ipfs/go-ipfs) on their machine.
This requirement may be lifted in the future when better infrastructure is set
up.

## Installation

```
$ go get -u github.com/whyrusleeping/gx
```

## Usage

Creating and publishing new generic package:

```bash
$ gx init
$ gx publish
```

This will output a 'package-hash' which is unique to the exact content of your
package at the time of publishing. If someone were to download your package and
republish it, it would produce the *exact* same hash.


## Dependencies
To add a dependency of another package to your package, simply import it by its
hash:

```bash
$ gx import QmaDFJvcHAnxpnMwcEh6VStYN4v4PB4S16j4pAuC2KSHVr
```

This downloads the package specified by the hash into the `vendor` directory in your
workspace. It also adds an entry referencing the package to the local `package.json`.

## Updating
Updating packages in gx is simple:

```bash
$ gx update mypkg QmbH7fpAV1FgMp6J7GZXUV6rj6Lck5tDix9JJGBSjFPgUd
```

This looks into your `package.json` for a dependency named `mypkg` and replaces
its hash reference with the one given.

Note, this will not touch your code at all, so any references to that hash you
have in your code will need to be updated. If you have a language tool
(e.g. `gx-go`) installed, and it has a `post-update` hook, references to the
given package should be updated correctly. If not, you may have to run sed over
the package to update everything. The bright side of that is that you are very
unlikely to have those hashes sitting around for any other reason so a global
find-replace should be just fine.

## Repos
gx supports named packages via user configured repositories. A repository is
simply an ipfs object whose links name package hashes. You can add a repository
as either an ipns or ipfs path.

### Usage

Add a new repo
```bash
$ gx repo add myrepo /ipns/QmPupmUqXHBxikXxuptYECKaq8tpGNDSetx1Ed44irmew3
```

List configured repos
```bash
$ gx repo list
myrepo       /ipns/QmPupmUqXHBxikXxuptYECKaq8tpGNDSetx1Ed44irmew3
```

List packages in a given repo
```bash
$ gx repo list myrepo
events      QmeJjwRaGJfx7j6LkPLjyPfzcD2UHHkKehDPkmizqSpcHT
smalltree   QmRgTZA6jGi49ipQxorkmC75d3pLe69N6MZBKfQaN6grGY
stump       QmebiJS1saSNEPAfr9AWoExvpfGoEK4QCtdLKCK4z6Qw7U
```

Import a package from a repo:
```bash
$ gx repo import events
```

## Hooks
gx can support a wide array of use cases by having sane defaults that are
extensible based on the scenario you are in. To this end, gx has hooks that
get called during certain operations.

These hooks are language specific, and gx will attempt to make calls to a
helper binary matching your language to execute the hooks, for example, when
writing go, gx calls `gx-go hook <hookname> <args>` for any given hook.

Currently available hooks are:

- `post-import`
  - called after a new package is imported and its info written to package.json
  - takes the hash of the newly imported package as an argument
- `post-init`
  - called after a new package is initialized
  - takes an optional argument of the directory of the newly init'ed package
- `pre-publish`
  - called during `gx publish` before the package is bundled up and added to ipfs
  - currently takes no arguments
- `post-publish`
  - called during `gx publish` after the package has been added to ipfs
  - takes the hash of the newly published package as an argument
- `post-update`
  - called during `gx update` after a dependency has been updated
  - takes the old package ref and the new hash as arguments

## The vendor directory

The `vendor/gx` (package) directory contains all of the downloaded dependencies of
your package.  You do not need to add the contents of the `vendor/gx` directory to
version control, simply running `gx install` in the root directory of your
project will fetch and download the appropriate versions of required packages. 

Note: This is not to say that you can't add the `vendor/gx` directory to version
control, by all means do if you want a single `git clone` or `svn co` to bring
all deps with it!

## Using gx as a Go package manager

If you want (like me) to use gx as a package manager for go, its pretty easy.
You will need the gx go extensions before starting your project:
```
$ go get -u github.com/whyrusleeping/gx-go
```

Once thats installed, use gx like normal to import dependencies.
You can import code from the vendor directory using:
```go
import "gx/<hash>/packagename"
```
for example:
```go
import "gx/QmR5FHS9TpLbL9oYY8ZDR3A7UWcHTBawU1FJ6pu9SvTcPa/cobra"
```
Then simply set the environment variable `GO15VENDOREXPERIMENT` to `1` and run
`go build` or `go install` like you normally would.

See [the gx-go repo](https://github.com/whyrusleeping/gx-go) for more details.

## Using gx as a package manager for language/environment X

If you want to extend gx to work with any other language or environment,
you can implement the relevant hooks in a binary named `gx-X` where the 'X'
is the name of your environment. (See 'hooks' above)

## Why is it called gx?

No reason. "gx" stands for nothing.

## License

MIT. Jeromy Johnson.