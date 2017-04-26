![gx logo](logo.jpeg)

# gx
> The language-agnostic, universal package manager

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://protocol.ai) [![](https://img.shields.io/badge/freenode-%23gx-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs,%23gx)

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
Users are encouraged to have a running [IPFS daemon](//github.com/ipfs/go-ipfs) of at least version 0.4.2 on their machines.
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

## Installing a gx package
If you've cloned down a gx package, simply run `gx install` or `gx i` to
install it (and its dependencies).

## Dependencies
To add a dependency of another package to your package, simply import it by its
hash:

```bash
$ gx import QmaDFJvcHAnxpnMwcEh6VStYN4v4PB4S16j4pAuC2KSHVr
```

This downloads the package specified by the hash into the `vendor` directory in your
workspace. It also adds an entry referencing the package to the local `package.json`.

Gx has a few nice tools to view and analyze dependencies. First off, the simple:

```bash
$ gx deps
go-log              QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52 1.2.0
go-libp2p-peer      QmWXjJo15p4pzT7cayEwZi2sWgJqLnGDof6ZGMh9xBgU1p 2.0.4
go-libp2p-peerstore QmYkwVGkwoPbMVQEbf6LonZg4SsCxGP3H7PBEtdNCNRyxD 1.2.5
go-testutil         QmYpVUnnedgGrp6cX2pBii5HRQgcSr778FiKVe7o7nF5Z3 1.0.2
go-ipfs-util        QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1 1.0.0
```

This just lists out the immediate dependencies of this package. To see
dependencies of dependencies, use the `-r` option: (and optionally the `-s`
option to sort them)

```bash
$ gx deps -r -s
go-base58           QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf 0.0.0
go-crypto           Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
go-datastore        QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU 1.0.0
go-ipfs-util        QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1 1.0.0
go-keyspace         QmUusaX99BZoELh7dmPgirqRQ1FAmMnmnBn3oiqDFGBUSc 1.0.0
go-libp2p-crypto    QmVoi5es8D5fNHZDqoW6DgDAEPEV5hQp8GBz161vZXiwpQ 1.0.4
go-libp2p-peer      QmWXjJo15p4pzT7cayEwZi2sWgJqLnGDof6ZGMh9xBgU1p 2.0.4
go-libp2p-peerstore QmYkwVGkwoPbMVQEbf6LonZg4SsCxGP3H7PBEtdNCNRyxD 1.2.5
go-log              QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52 1.2.0
go-logging          QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV 0.0.0
go-multiaddr        QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd 0.0.0
go-multiaddr-net    QmY83KqqnQ286ZWbV2x7ixpeemH3cBpk8R54egS619WYff 1.3.0
go-multihash        QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku 0.0.0
go-net              QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt 0.0.0
go-testutil         QmYpVUnnedgGrp6cX2pBii5HRQgcSr778FiKVe7o7nF5Z3 1.0.2
go-text             Qmaau1d1WjnQdTYfRYfFVsCS97cgD8ATyrKuNoEfexL7JZ 0.0.0
go.uuid             QmcyaFHbyiZfoX5GTpcqqCPYmbjYNAhRDekXSJPFHdYNSV 1.0.0
gogo-protobuf       QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV 0.0.0
goprocess           QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP 1.0.0
mafmt               QmeLQ13LftT9XhNn22piZc3GP56fGqhijuL5Y8KdUaRn1g 1.1.1
```

That's pretty useful, I now know the full set of packages my package depends on.
But what's difficult now is being able to tell what is imported where. To
address that, gx has a `--tree` option:

```bash
$ gx deps --tree
├─ go-base58          QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf 0.0.0
├─ go-multihash       QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku 0.0.0
│  ├─ go-base58       QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf 0.0.0
│  └─ go-crypto       Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
├─ go-ipfs-util       QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1 1.0.0
│  ├─ go-base58       QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf 0.0.0
│  └─ go-multihash    QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku 0.0.0
│     ├─ go-base58    QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf 0.0.0
│     └─ go-crypto    Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
├─ go-log             QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR 1.1.2
│  ├─ randbo          QmYvsG72GsfLgUeSojXArjnU6L4Wmwk7wuAxtNLuyXcc1T 0.0.0
│  ├─ go-net          QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt 0.0.0
│  │  ├─ go-text      Qmaau1d1WjnQdTYfRYfFVsCS97cgD8ATyrKuNoEfexL7JZ 0.0.0
│  │  └─ go-crypto    Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
│  └─ go-logging      QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV 0.0.0
└─ go-libp2p-crypto   QmUEUu1CM8bxBJxc3ZLojAi8evhTr4byQogWstABet79oY 1.0.2
   ├─ gogo-protobuf   QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV 0.0.0
   ├─ go-log          Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH 0.0.0
   │  ├─ go.uuid      QmPC2dW6jyNzzBKYuHLBhxzfWaUSkyC9qaGMz7ciytRSFM 0.0.0
   │  ├─ go-logging   QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV 0.0.0
   │  ├─ go-net       QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt 0.0.0
   │  │  ├─ go-text   Qmaau1d1WjnQdTYfRYfFVsCS97cgD8ATyrKuNoEfexL7JZ 0.0.0
   │  │  └─ go-crypto Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
   │  └─ randbo       QmYvsG72GsfLgUeSojXArjnU6L4Wmwk7wuAxtNLuyXcc1T 0.0.0
   ├─ go-ipfs-util    QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1 1.0.0
   │  ├─ go-base58    QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf 0.0.0
   │  └─ go-multihash QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku 0.0.0
   │     ├─ go-base58 QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf 0.0.0
   │     └─ go-crypto Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
   └─ go-msgio        QmRQhVisS8dmPbjBUthVkenn81pBxrx1GxE281csJhm2vL 0.0.0
      └─ go-randbuf   QmYNGtJHgaGZkpzq8yG6Wxqm6EQTKqgpBfnyyGBKbZeDUi 0.0.0
```

Now you can see the *entire* tree of dependencies for this project. Although,
for larger projects, this will get messy. If you're just interested in the
dependency tree of a single package, you can use the `--highlight` option
to filter the trees printing:

```bash
$ gx deps --tree --highlight=go-crypto
├─ go-multihash       QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku 0.0.0
│  └─ go-crypto       Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
├─ go-ipfs-util       QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1 1.0.0
│  └─ go-multihash    QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku 0.0.0
│     └─ go-crypto    Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
├─ go-log             QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR 1.1.2
│  └─ go-net          QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt 0.0.0
│     └─ go-crypto    Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
└─ go-libp2p-crypto   QmUEUu1CM8bxBJxc3ZLojAi8evhTr4byQogWstABet79oY 1.0.2
   ├─ go-log          Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH 0.0.0
   │  └─ go-net       QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt 0.0.0
   │     └─ go-crypto Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
   └─ go-ipfs-util    QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1 1.0.0
      └─ go-multihash QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku 0.0.0
         └─ go-crypto Qme1boxspcQWR8FBzMxeppqug2fYgYc15diNWmqgDVnvn2 0.0.0
```

This tree is a subset of the previous one, filtered to only show leaves that
end in the selected package.

The gx deps command also has two other smaller subcommands, `dupes` and
`stats`. `gx deps dupes` will print out packages that are imported multiple
times with the same name, but different hashes. This can be useful to see if
different versions of the same package have been imported in different places
in the dependency tree. Allowing the user to more easily go and address the
discrepancy. `gx deps stats` will output the total number of packages imported
(total and unique) as well as the average depth of imports in the tree. This
can be used to give you a rough idea of the complexity of your package.

### The gx dependency graph manifesto
I firmly believe that packages are better when:

#### 1. The depth of the dependency tree is minimized.
This means restructuring your code in such a way that flattens (and perhaps
widens as a consequence) the tree. For example, in Go, this often times means
making an interface its own package, and implementations into their own
separate packages. The benefits here are that flatter trees are far easier to
update. For every package deep a dependency is, you have to update, test,
commit, review and merge another package. That's a lot of work, and also a lot
of extra room for problems to sneak in.

#### 2. The width of the tree is minimized, but not at the cost of increasing depth.
This should be fairly common sense, but striving to import packages only where
they are actually needed helps to improve code quality. Imagine having a helper
function in one package, simply because it's convenient to have it there, that
depends on a bunch of other imports from elsewhere in the tree. Sure it's nice,
and doesn't actually increase the 'total' number of packages you depend on. But
now you've created an extra batch of work for you to do any time any of these
are updated, and you also now force anyone who wants to import the package with
your helper function to also import all those other dependencies.

Adhering to the above two rules should (I'm very open to discussion on this)
improve overall code quality, and make your codebase far easier to navigate and
work on.

## Updating
Updating packages in gx is simple:

```bash
$ gx update mypkg QmbH7fpAV1FgMp6J7GZXUV6rj6Lck5tDix9JJGBSjFPgUd
```

This looks into your `package.json` for a dependency named `mypkg` and replaces
its hash reference with the one given.

Alternatively, you can just specify the hash you want to update to:

```bash
$ gx update QmbH7fpAV1FgMp6J7GZXUV6rj6Lck5tDix9JJGBSjFPgUd
```

Doing it this way will pull down the package, check its name, and then update
that dependency.

Note that by default, this will not touch your code at all, so any references
to that hash you have in your code will need to be updated. If you have a
language tool (e.g. `gx-go`) installed, and it has a `post-update` hook,
references to the given package should be updated correctly. If not, you may
have to run sed over the package to update everything. The bright side of that
is that you are very unlikely to have those hashes sitting around for any other
reason so a global find-replace should be just fine.

## Publishing and Releasing
Gx by default will not let you publish a package twice if you haven't updated
its version. To get around this, you can pass the `-f` flag. Though this is not
recommended, it's still perfectly possible to do.

To update the version easily, use the `gx version` subcommand. You can either set the version manually:

```bash
$ gx version 5.11.4
```

Or just do a 'version bump':

```bash
$ gx version patch
updated version to: 5.11.5
$ gx version minor
updated version to: 5.12.0
$ gx version major
updated version to: 6.0.0
```

Most of the time, your process will look something like:

```bash
$ gx version minor
updated version to: 6.1.0
$ gx publish
package whys-awesome-package published with hash: QmaoaEi6uNMuuXKeYcXM3gGUEQLzbDWGcFUdd3y49crtZK
$ git commit -a -m "gx publish 6.1.0"
[master 5c4d36c] gx publish 6.1.0
 2 files changed, 3 insertions(+), 2 deletions(-)
```

To automate this, you can use the `release` subcommand. `gx release <version>`
will automatically do a version update (using the same inputs as the normal
`version` command), run a `gx publish`, and then execute whatever you have set
in your `package.json` as your `releaseCmd`. To get the above git commit flow,
you can set it to: `git commit -a -m \"gx publish $VERSION\"` and gx will
replace `$VERSION` with the newly changed version before executing the git
commit.

### Ignoring files from a publish
You can use a `.gxignore` file to make gx ignore certain files during a publish.
This has the same behaviour as a `.gitignore`.

Gx also respects a `.gitignore` file if present, and will not publish any file
excluded by it.


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

Gx by default will install packages 'globally' in the global install location
for your given project type.  Global gx packages are shared across all packages
that depend on them.  The location of this directory is not set in stone, if
for your specific environment you'd like it somewhere else, simply add a hook
to your environments extension tool named `install-path` (see above) and gx
will use that path instead. If your language does not set a global install
path, gx will fallback to installing locally as the default.  This means that
it will create a folder in the current directory named `vendor` and install
things to it.

When running `gx install` in the directory of your package, gx will recursively
fetch all of the dependencies specified in the `package.json` and save them to
the install path specified.

Gx supports both local and global installation paths. Since the default is
global, to install locally, use `--local` or `--global=false`.  The global flag
is passed to the `install-path` hook for your extension code to use in its
logic.

## Using gx as a Go package manager

If you want (like me) to use gx as a package manager for go, it's pretty easy.
You will need the gx go extensions before starting your project:
```
$ go get -u github.com/whyrusleeping/gx-go
```

Once that's installed, use gx like normal to import dependencies.
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
