![gx logo](logo.jpeg)

# gx
> The language-agnostic, universal package manager

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io) [![](https://img.shields.io/badge/freenode-%23gx-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs,%23gx)

gx is a packaging tool built around the distributed, content addressed filesystem
[IPFS](//github.com/ipfs/ipfs). It aims to be flexible, powerful and simple.

gx is **Alpha Quality**. It's not perfect yet, but it's proven dependable enough
for managing dependencies in [go-ipfs](https://github.com/ipfs/go-ipfs/) and
ready for pioneering developers and early users to try out and explore.

## Table of Contents
- [Background](#background)
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

## Background

gx was originally designed to handle dependencies in Go projects in a
distributed fashion, and pulls ideas from other beloved package managers (like
[npm](http://npmjs.org/)).

gx was designed with the following major goals in mind:

1. Be language/ecosystem agnostic by providing [git-like hooks](https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks) for adding [new ecosystems](https://github.com/whyrusleeping/gx-go).
2. Provide completely reproducible packages through content addressing.
3. Use [a flexible, distributed storage backend](http://ipfs.io/).


## Requirements
Users are encouraged to have a running [IPFS daemon](//github.com/ipfs/go-ipfs) of at least version 0.4.0 on their machines.
If not present, gx will use the public gateway.
If you wish to publish a package, a local running daemon is a hard requirement.


## Installation

```
$ go get -u github.com/whyrusleeping/gx
```

This will download the source into `$GOPATH/src/github.com/whyrusleeping/gx`
and build and install a binary to `$GOPATH/bin`. To modify gx, just change
the source in that directory, and run `go build`.

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
  - called after a new package is imported and its info written to package.json.
  - takes the hash of the newly imported package as an argument.
- `post-init`
  - called after a new package is initialized.
  - takes an optional argument of the directory of the newly init'ed package.
- `pre-publish`
  - called during `gx publish` before the package is bundled up and added to ipfs.
  - currently takes no arguments.
- `post-publish`
  - called during `gx publish` after the package has been added to ipfs.
  - takes the hash of the newly published package as an argument.
- `post-update`
  - called during `gx update` after a dependency has been updated.
  - takes the old package ref and the new hash as arguments.
- `post-install`
  - called after a new package is downloaded, during install and import.
  - takes the path to the new package as an argument.
- `install-path`
  - called during package installs and imports.
  - sets the location for gx to install packages to.

## Package directories

Gx by default will install packages 'locally'. This means that it will create a
folder in the current directory named `vendor` and install things to it. When
running `gx install` in the directory of your package will recursively fetch
all of the dependencies specified in the `package.json` and save them to the
local package directory.

The location of this directory is not set in stone, if for your specific
environment you'd like it somewhere else, simply add a hook to your environments
extension tool named `install-path` (see above) and gx will use that path
instead.

Gx also supports a global installation path, to set this one you must handle
the `--global` flag on your `install-path` hook. Global gx packages are shared
across all packages that depend on them.

## Ignoring files from a publish
You can use a `.gxignore` file to make gx ignore certain files during a publish.
This has the same behaviour as a `.gitignore`.

Gx also respects a `.gitignore` file if present, and will not publish any file
excluded by it.

## Using gx as a Go package manager

If you want (like me) to use gx as a package manager for go, its pretty easy.
You will need the gx go extensions before starting your project:
```
$ go get -u github.com/whyrusleeping/gx-go
```

Once thats installed, use gx like normal to import dependencies.
You can import code from the vendor directory using:
```go
import "gx/ipfs/<hash>/packagename"
```

for example, if i have a package foobar, you can import with gx it like so:
```bash
$ gx import QmR5FHS9TpLbL9oYY8ZDR3A7UWcHTBawU1FJ6pu9SvTcPa
```

And then in your go code, you can use it with:
```go
import "gx/ipfs/QmR5FHS9TpLbL9oYY8ZDR3A7UWcHTBawU1FJ6pu9SvTcPa/foobar"
```

Then simply set the environment variable `GO15VENDOREXPERIMENT` to `1` and run
`go build` or `go install` like you normally would. Alternatively, install
your dependencies globally (`gx install --global`) and you can leave off the
environment variable part.

See [the gx-go repo](https://github.com/whyrusleeping/gx-go) for more details.

## Using gx as a package manager for language/environment X

If you want to extend gx to work with any other language or environment, you
can implement the relevant hooks in a binary named `gx-X` where the 'X' is the
name of your environment. After that, any package whose language is set to 'X'
will call out to that tools hooks during normal `gx` operations. For example, a
'go' package would call `gx-go hook pre-publish` during a `gx publish`
invocation before the package is actually published.  For more information on
hooks, check out the hooks section above.

## Why is it called gx?

No reason. "gx" stands for nothing.

## Getting Involved

If you're interested in gx, please stop by #gx and #ipfs on freenode irc!

## License

MIT. Jeromy Johnson.
