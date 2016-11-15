package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/blang/semver"
	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
	log "github.com/whyrusleeping/stump"

	"github.com/whyrusleeping/json-filter"
)

var (
	vendorDir = filepath.Join("vendor", "gx", "ipfs")
	cwd       string
	pm        *gx.PM
)

const PkgFileName = gx.PkgFileName

func LoadPackageFile(path string) (*gx.Package, error) {
	if path == PkgFileName {
		root, err := gx.GetPackageRoot()
		if err != nil {
			return nil, err
		}

		path = filepath.Join(root, PkgFileName)
	}

	var pkg gx.Package
	err := gx.LoadPackageFile(&pkg, path)
	if err != nil {
		return nil, err
	}

	if pkg.GxVersion == "" {
		pkg.GxVersion = gx.GxVersion
	}

	if pkg.SubtoolRequired {
		found, err := gx.IsSubtoolInstalled(pkg.Language)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("package requires a subtool (gx-%s) and none was found.", pkg.Language)
		}
	}

	return &pkg, nil
}

func main() {
	cfg, err := gx.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	pm, err = gx.NewPM(cfg)
	if err != nil {
		log.Fatal(err)
	}

	app := cli.NewApp()
	app.Author = "whyrusleeping"
	app.Version = gx.GxVersion
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "print verbose logging information",
		},
	}
	app.Before = func(c *cli.Context) error {
		log.Verbose = c.Bool("verbose")

		gcwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cwd = gcwd

		return nil
	}

	app.Usage = "gx is a packaging tool that uses ipfs"

	app.Commands = []cli.Command{
		CleanCommand,
		DepsCommand,
		GetCommand,
		ImportCommand,
		DiffCommand,
		InitCommand,
		InstallCommand,
		PublishCommand,
		ReleaseCommand,
		RepoCommand,
		UpdateCommand,
		VersionCommand,
		ViewCommand,
		SetCommand,
		TestCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func checkLastPubVer() string {
	out, err := ioutil.ReadFile(filepath.Join(cwd, ".gx", "lastpubver"))
	if err != nil {
		return ""
	}

	parts := bytes.Split(out, []byte{':'})
	return string(parts[0])
}

var PublishCommand = cli.Command{
	Name:  "publish",
	Usage: "publish a package",
	Description: `publish a package into ipfs using a locally running daemon.

'publish' bundles up all files associated with the package (respecting
.gitignore and .gxignore files), adds them to ipfs, and writes out the
resulting package hash.

By default, you cannot publish a package without updating the version
number. This is a soft requirement and can be skipped by specifying the
-f or --force flag.
`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force,f",
			Usage: "allow publishing without bumping version",
		},
	},
	Action: func(c *cli.Context) error {
		if gx.UsingGateway {
			log.Log("gx cannot publish using public gateways.")
			log.Log("please run an ipfs node and try again.")
			return nil
		}

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		if !c.Bool("force") {
			if pkg.Version == checkLastPubVer() {
				log.Fatal("please update your packages version before publishing. (use -f to skip)")
			}
		}

		return doPublish(pkg)
	},
}

func doPublish(pkg *gx.Package) error {
	err := gx.TryRunHook("pre-publish", pkg.Language, pkg.SubtoolRequired)
	if err != nil {
		return err
	}

	hash, err := pm.PublishPackage(cwd, &pkg.PackageBase)
	if err != nil {
		return fmt.Errorf("publishing: %s", err)
	}
	log.Log("package %s published with hash: %s", pkg.Name, hash)

	// write out version hash
	err = writeLastPub(pkg.Version, hash)
	if err != nil {
		return err
	}

	err = gx.TryRunHook("post-publish", pkg.Language, pkg.SubtoolRequired, hash)
	if err != nil {
		return err
	}

	return nil
}

func writeLastPub(vers string, hash string) error {
	err := os.MkdirAll(".gx", 0755)
	if err != nil {
		return err
	}

	fi, err := os.Create(".gx/lastpubver")
	if err != nil {
		return fmt.Errorf("failed to create version file: %s", err)
	}

	defer fi.Close()

	log.VLog("writing published version to .gx/lastpubver")
	_, err = fmt.Fprintf(fi, "%s: %s\n", vers, hash)
	if err != nil {
		return fmt.Errorf("failed to write version file: %s", err)
	}

	return nil
}

var ImportCommand = cli.Command{
	Name:  "import",
	Usage: "import a package as a dependency",
	Flags: []cli.Flag{
		cli.BoolTFlag{
			Name:  "global",
			Usage: "download imported package to global store",
		},
		cli.BoolFlag{
			Name:  "local",
			Usage: "install packages locally (equal to --global=false)",
		},
	},
	Action: func(c *cli.Context) error {
		if len(c.Args()) == 0 {
			return fmt.Errorf("import requires a package name")
		}

		global := c.BoolT("global")
		if c.Bool("local") {
			global = false
		}

		pm.SetGlobal(global)

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		depname := c.Args().First()
		cdep := pkg.FindDep(depname)
		if cdep != nil {
			return fmt.Errorf("package %s already imported as %s", cdep.Hash, cdep.Name)
		}

		dephash, err := pm.ResolveDepName(depname)
		if err != nil {
			return err
		}

		ipath, err := gx.InstallPath(pkg.Language, "", global)
		if err != nil {
			return err
		}

		npkg, err := pm.InstallPackage(dephash, ipath)
		if err != nil {
			return fmt.Errorf("(install):", err)
		}

		if pkg.FindDep(npkg.Name) != nil {
			s := fmt.Sprintf("package with name %s already imported, continue?", npkg.Name)
			if !yesNoPrompt(s, false) {
				return nil
			}
			log.Log("continuing, please note some things may not work as expected")
		}

		ndep := &gx.Dependency{
			Author:  npkg.Author,
			Hash:    dephash,
			Name:    npkg.Name,
			Version: npkg.Version,
		}

		pkg.Dependencies = append(pkg.Dependencies, ndep)
		err = gx.SavePackageFile(pkg, PkgFileName)
		if err != nil {
			return fmt.Errorf("writing pkgfile: %s", err)
		}

		err = gx.TryRunHook("post-import", npkg.Language, npkg.SubtoolRequired, dephash)
		if err != nil {
			return fmt.Errorf("running post-import: %s", err)
		}

		return nil
	},
}

var InstallCommand = cli.Command{
	Name:    "install",
	Usage:   "install this package",
	Aliases: []string{"i"},
	Flags: []cli.Flag{
		cli.BoolTFlag{
			Name:  "global",
			Usage: "install package in global namespace",
		},
		cli.BoolFlag{
			Name:  "local",
			Usage: "install packages locally (equal to --global=false)",
		},
		cli.BoolFlag{
			Name:  "save",
			Usage: "write installed packages as deps in package.json",
		},
	},
	Action: func(c *cli.Context) error {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		save := c.Bool("save")

		global := c.BoolT("global")
		if c.Bool("local") {
			global = false
		}

		pm.SetGlobal(global)

		if len(c.Args()) == 0 {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			err = gx.TryRunHook("req-check", pkg.Language, pkg.SubtoolRequired, cwd)
			if err != nil {
				return err
			}

			ipath, err := gx.InstallPath(pkg.Language, cwd, global)
			if err != nil {
				return err
			}

			err = pm.InstallDeps(pkg, ipath)
			if err != nil {
				return fmt.Errorf("install deps:", err)
			}
			return nil
		}

		ipath, err := gx.InstallPath(pkg.Language, "", global)
		if err != nil {
			return err
		}

		for _, p := range c.Args() {
			phash, err := pm.ResolveDepName(p)
			if err != nil {
				return fmt.Errorf("resolving package '%s': %s", p, err)
			}

			if p != phash {
				log.VLog("%s resolved to %s", p, phash)
			}

			ndep, err := pm.ImportPackage(ipath, p)
			if err != nil {
				return fmt.Errorf("importing package '%s': %s", p, err)
			}

			if save {
				pkg.Dependencies = append(pkg.Dependencies, ndep)
			}
		}

		if save {
			err := gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var GetCommand = cli.Command{
	Name:  "get",
	Usage: "download a package",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "o",
			Usage: "specify output dir name",
		},
	},
	Action: func(c *cli.Context) error {
		if !c.Args().Present() {
			return fmt.Errorf("no package specified")
		}

		pkg, err := pm.ResolveDepName(c.Args().First())
		if err != nil {
			return err
		}

		out := c.String("o")
		if out == "" {
			out = filepath.Join(cwd, pkg)
		}

		log.Log("writing package to:", out)
		_, err = pm.GetPackageTo(pkg, out)
		if err != nil {
			return fmt.Errorf("fetching package: %s", err)
		}
		return nil
	},
}

var InitCommand = cli.Command{
	Name:  "init",
	Usage: "initialize a package in the current working directory",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "lang",
			Usage: "specify primary language of new package",
		},
	},
	Action: func(c *cli.Context) error {
		var pkgname string
		if len(c.Args()) > 0 {
			pkgname = c.Args().First()
		} else {
			pkgname = filepath.Base(cwd)
		}

		lang := c.String("lang")
		if !c.IsSet("lang") {
			lang = promptUser("what language will the project be in?")
		}

		log.Log("initializing package %s...", pkgname)
		err := pm.InitPkg(cwd, pkgname, lang, func(p *gx.Package) {
			p.Bugs.Url = promptUser("where should users go to report issues?")
		})

		if err != nil {
			return fmt.Errorf("init error: %s", err)
		}

		return nil
	},
}

var UpdateCommand = cli.Command{
	Name:      "update",
	Usage:     "update a package dependency",
	ArgsUsage: "[oldref] [newref]",
	Description: `Update a package to a specified ref.

EXAMPLE:
   Update 'myPkg' to a given version (referencing it by package name):

   $ gx update myPkg QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1

   or reference it by hash:

   $ export OLDHASH=QmdTTcAwxWhHLruoZtowxuqua1e5GVkYzxziiYPDn4vWJb
   $ export NEWHASH=QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1
   $ gx update $OLDHASH $NEWHASH
`,
	Flags: []cli.Flag{
		cli.BoolTFlag{
			Name:  "global",
			Usage: "install new package in global namespace",
		},
		cli.BoolFlag{
			Name:  "local",
			Usage: "install packages locally (equal to --global=false)",
		},
		cli.BoolFlag{
			Name:  "with-deps",
			Usage: "experimental feature to recursively update child deps too",
		},
	},
	Action: func(c *cli.Context) error {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			log.Fatal("error: ", err)
		}

		var existing, target string
		switch len(c.Args()) {
		case 0:
			log.Fatal("update requires two arguments, current and target")
		case 1:
			target = c.Args()[0]
		case 2:
			existing = c.Args()[0]
			target = c.Args()[1]
		default:
			log.Log("ignoring extra arguments: %s", c.Args()[2:])
		}

		trgthash, err := pm.ResolveDepName(target)
		if err != nil {
			return err
		}

		global := c.BoolT("global")
		if c.Bool("local") {
			global = false
		}

		ipath, err := gx.InstallPath(pkg.Language, cwd, global)
		if err != nil {
			return err
		}

		npkg, err := pm.InstallPackage(trgthash, ipath)
		if err != nil {
			log.Fatal("(installpackage) : ", err)
		}

		if existing == "" {
			existing = npkg.Name
		}

		var oldhash string
		olddep := pkg.FindDep(existing)
		if olddep == nil {
			log.Fatal("unknown package: ", existing)
		}
		oldhash = olddep.Hash

		log.Log("updating %s to version %s (%s)", olddep.Name, npkg.Version, trgthash)

		if npkg.Name != olddep.Name {
			prompt := fmt.Sprintf(`Target package has a different name than new package:
old: %s (%s)
new: %s (%s)
continue?`, olddep.Name, olddep.Hash, npkg.Name, trgthash)
			if !yesNoPrompt(prompt, false) {
				log.Fatal("refusing to update package with different names")
			}
		}

		log.VLog("running pre update hook...")
		err = gx.TryRunHook("pre-update", pkg.Language, pkg.SubtoolRequired, existing)
		if err != nil {
			return err
		}

		if c.Bool("with-deps") {
			err := RecursiveDepUpdate(pkg, oldhash, trgthash)
			if err != nil {
				return err
			}
		} else {
			log.VLog("checking for potential package naming collisions...")
			err = updateCollisionCheck(pkg, olddep, trgthash, nil, make(map[string]struct{}))
			if err != nil {
				log.Fatal("update sanity check: ", err)
			}
			log.VLog("  - no collisions found for updated package")
		}

		olddep.Hash = trgthash
		olddep.Version = npkg.Version

		err = gx.SavePackageFile(pkg, PkgFileName)
		if err != nil {
			return fmt.Errorf("writing package file: %s", err)
		}

		log.VLog("running post update hook...")
		err = gx.TryRunHook("post-update", pkg.Language, pkg.SubtoolRequired, oldhash, trgthash)
		if err != nil {
			return err
		}

		log.VLog("update complete!")

		return nil
	},
}

func updateCollisionCheck(ipkg *gx.Package, idep *gx.Dependency, trgt string, chain []string, skip map[string]struct{}) error {
	return ipkg.ForEachDep(func(dep *gx.Dependency, pkg *gx.Package) error {
		if _, ok := skip[dep.Hash]; ok {
			return nil
		}

		if dep == idep {
			return nil
		}
		skip[dep.Hash] = struct{}{}

		if (dep.Name == idep.Name && dep.Hash != trgt) || (dep.Hash == idep.Hash && dep.Name != idep.Name) {
			log.Log("dep %s also imports %s (%s)", strings.Join(chain, "/"), dep.Name, dep.Hash)
			return nil
		}

		return updateCollisionCheck(pkg, idep, trgt, append(chain, dep.Name), skip)
	})
}

var VersionCommand = cli.Command{
	Name:  "version",
	Usage: "view or modify this package's version",
	Description: `view or modify this package's version

   run without any arguments, will print the current semver of this package.

   if an argument is given, it will be parsed as a semver; if that succeeds,
   the version will be set to that exactly. If the argument is not a semver,
   it should be one of three things: "major", "minor", or "patch". Passing
   any of those three will bump the corresponding segment of the semver up
   by one.

EXAMPLE:

   > gx version
   0.4.0

   > gx version patch
   updated version to 0.4.1

   > gx version major
   updated version to 1.0.0

   > gx version 2.5.7
   updated version to 2.5.7
`,
	Action: func(c *cli.Context) (outerr error) {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}
		if !c.Args().Present() {
			fmt.Println(pkg.Version)
			return
		}

		return updateVersion(pkg, c.Args().First())
	},
}

func updateVersion(pkg *gx.Package, nver string) (outerr error) {
	if nver == "" {
		return fmt.Errorf("must specify version with non-zero length")
	}

	defer func() {
		err := gx.SavePackageFile(pkg, PkgFileName)
		if err != nil {
			outerr = err
		}
	}()

	// if argument is a semver, set version to it
	_, err := semver.Make(nver)
	if err == nil {
		pkg.Version = nver
		return nil
	}

	v, err := semver.Make(pkg.Version)
	if err != nil {
		return err
	}
	switch nver {
	case "major":
		v.Major++
		v.Minor = 0
		v.Patch = 0
		v.Pre = nil // reset prerelase info
	case "minor":
		v.Minor++
		v.Patch = 0
		v.Pre = nil
	case "patch":
		v.Patch++
		v.Pre = nil
	default:
		if nver[0] == 'v' {
			nver = nver[1:]
		}
		newver, err := semver.Make(nver)
		if err != nil {
			log.Error(err)
			return
		}
		v = newver
	}
	log.Log("updated version to: %s", v)

	pkg.Version = v.String()

	return nil
}

var ViewCommand = cli.Command{
	Name:  "view",
	Usage: "view package information",
	Description: `view can be used to print out information in the package.json
   of this package, or a dependency specified either by name or hash.

EXAMPLE:
   > gx view language
   go

   > gx view .
   {
     "language": "go",
     "name": "gx",
     "version": "0.2.0
   }

   > gx view go-libp2p gx.dvcsimport
   "github.com/ipfs/go-libp2p"

   > gx view '.gxDependencies[0].name'
   go-multihash

   > gx view '.gxDependencies[.name=go-multiaddr].hash'
   QmWLfU4tstw2aNcTykDm44xbSTCYJ9pUJwfhQCKGwckcHx
`,
	Action: func(c *cli.Context) error {
		if !c.Args().Present() {
			log.Fatal("must specify at least a query")
		}

		var cfg map[string]interface{}
		if len(c.Args()) == 2 {
			pkg, err := LoadPackageFile(gx.PkgFileName)
			if err != nil {
				return err
			}

			ref := c.Args()[0]
			dep := pkg.FindDep(ref)
			if dep == nil {
				return fmt.Errorf("no dep referenced by %s", ref)
			}
			err = gx.LoadPackage(&cfg, pkg.Language, dep.Hash)
			if err != nil {
				return err
			}
		} else {
			root, err := gx.GetPackageRoot()
			if err != nil {
				return err
			}

			err = gx.LoadPackageFile(&cfg, filepath.Join(root, PkgFileName))
			if err != nil {
				return err
			}
		}

		queryStr := c.Args()[len(c.Args())-1]
		val, err := filter.Get(cfg, queryStr)
		if err != nil {
			return err
		}

		jsonPrint(val)
		return nil
	},
}

var CleanCommand = cli.Command{
	Name:  "clean",
	Usage: "cleanup unused packages in vendor directory",
	Description: `deletes any package in the 'vendor/gx' directory
   that is not a dependency of this package.

   use '--dry-run' to print packages that would be deleted without actually
   removing them.
   `,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run",
			Usage: "print out things to be removed without removing them",
		},
	},
	Action: func(c *cli.Context) error {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		dry := c.Bool("dry-run")

		good, err := pm.EnumerateDependencies(pkg)
		if err != nil {
			return err
		}

		ipath, err := gx.InstallPath(pkg.Language, cwd, false)
		if err != nil {
			return err
		}

		vdir := filepath.Join(ipath, "gx", "ipfs")
		dirinfos, err := ioutil.ReadDir(vdir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		for _, di := range dirinfos {
			if !strings.HasPrefix(di.Name(), "Qm") {
				continue
			}
			_, keep := good[di.Name()]
			if !keep {
				fmt.Println(di.Name())
				if !dry {
					err := os.RemoveAll(filepath.Join(vdir, di.Name()))
					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	},
}

var DepsCommand = cli.Command{
	Name:  "deps",
	Usage: "print out package dependencies",
	Description: `prints out dependencies for this package

   Run with no flags, will print out name, hash, and version for each
   package that is a direct dependency of this package.

   The '-r' option will recursively print out all dependencies directly
   and indirectly required by this package.

   The '--tree' option will do the same as '-r', but will add indents
   to show which packages are dependent on which other.
`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "r",
			Usage: "print deps recursively",
		},
		cli.BoolFlag{
			Name:  "q",
			Usage: "only print hashes",
		},
		cli.BoolFlag{
			Name:  "tree",
			Usage: "print deps as a tree",
		},
		cli.BoolTFlag{
			Name:  "s,sort",
			Usage: "sort output by package name",
		},
		cli.StringFlag{
			Name:  "highlight",
			Usage: "for tree printing, prune branches unrelated to arg",
		},
		cli.BoolFlag{
			Name:  "collapse",
			Usage: "for tree printing, prune branches already printed",
		},
	},
	Subcommands: []cli.Command{
		depBundleCommand,
		depFindCommand,
		depStatsCommand,
		depDupesCommand,
	},
	Action: func(c *cli.Context) error {
		rec := c.Bool("r")
		quiet := c.Bool("q")

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		if c.Bool("tree") {
			dt, err := genDepsTree(pm, pkg)
			if err != nil {
				log.Fatal(err)
			}

			dt.printFiltered(c.String("highlight"), quiet, c.Bool("collapse"))
			return nil
		}

		var deps []string
		if rec {
			depmap, err := pm.EnumerateDependencies(pkg)
			if err != nil {
				return err
			}

			for k, _ := range depmap {
				deps = append(deps, k)
			}
		} else {
			for _, d := range pkg.Dependencies {
				deps = append(deps, d.Hash)
			}
		}

		sort.Strings(deps)

		buf := new(bytes.Buffer)
		w := tabwriter.NewWriter(buf, 12, 4, 1, ' ', 0)
		for _, d := range deps {
			if !quiet {
				var dpkg gx.Package
				err := gx.LoadPackage(&dpkg, pkg.Language, d)
				if err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("package %s not found", d)
					}
					return err
				}

				fmt.Fprintf(w, "%s\t%s\t%s\n", dpkg.Name, d, dpkg.Version)
			} else {
				fmt.Fprintln(w, d)
			}
		}
		w.Flush()

		if c.Bool("sort") {
			lines := strings.Split(buf.String(), "\n")
			lines = lines[:len(lines)-1] // remove trailing newline
			sort.Strings(lines)
			for _, l := range lines {
				fmt.Println(l)
			}
		} else {
			io.Copy(os.Stdout, buf)
		}

		return nil
	},
}

var depFindCommand = cli.Command{
	Name:  "find",
	Usage: "print hash of a given dependency",
	Action: func(c *cli.Context) error {

		if len(c.Args()) != 1 {
			fmt.Errorf("must be passed exactly one argument")
		}

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		dep := c.Args()[0]

		for _, d := range pkg.Dependencies {
			if d.Name == dep {
				fmt.Println(d.Hash)
				return nil
			}
		}
		log.Fatal("no dependency named '%s' found", dep)

		return nil
	},
}

var depDupesCommand = cli.Command{
	Name:  "dupes",
	Usage: "print out packages with same names, but different hashes",
	Action: func(c *cli.Context) error {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		depmap, err := pm.EnumerateDependencies(pkg)
		if err != nil {
			return err
		}

		byname := make(map[string]string)
		for hash, name := range depmap {
			h, ok := byname[name]
			if ok {
				fmt.Printf("package %s imported as both %s and %s\n", name, h, hash)
				continue
			}

			byname[name] = hash
		}

		return nil
	},
}

var depStatsCommand = cli.Command{
	Name:  "stats",
	Usage: "print out statistics about this packages dependency tree",
	Action: func(c *cli.Context) error {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		ds, err := gx.GetDepStats(pkg)
		if err != nil {
			return err
		}

		fmt.Printf("Total Import Count: %d\n", ds.TotalCount)
		fmt.Printf("Unique Import Count: %d\n", ds.TotalUnique)
		fmt.Printf("Average Import Depth: %.2f\n", ds.AverageDepth)

		return nil
	},
}

var depBundleCommand = cli.Command{
	Name:  "bundle",
	Usage: "print hash of object containing all dependencies for this package",
	Action: func(c *cli.Context) error {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		obj, err := depBundleForPkg(pkg)
		if err != nil {
			return err
		}

		fmt.Println(obj)
		return nil
	},
}

func depBundleForPkg(pkg *gx.Package) (string, error) {
	return depBundleForPkgRec(pkg, make(map[string]bool))
}

func depBundleForPkgRec(pkg *gx.Package, done map[string]bool) (string, error) {
	obj, err := pm.Shell().NewObject("unixfs-dir")
	if err != nil {
		return "", err
	}

	for _, dep := range pkg.Dependencies {
		if done[dep.Hash] {
			continue
		}

		log.Log("processing dep: ", dep.Name)
		nobj, err := pm.Shell().PatchLink(obj, dep.Name+"-"+dep.Hash, dep.Hash, false)
		if err != nil {
			return "", err
		}

		var cpkg gx.Package
		err = gx.LoadPackage(&cpkg, pkg.Language, dep.Hash)
		if err != nil {
			return "", err
		}

		child, err := depBundleForPkgRec(&cpkg, done)
		if err != nil {
			return "", err
		}

		nobj, err = pm.Shell().PatchLink(nobj, dep.Name+"-"+dep.Hash+"-deps", child, false)
		if err != nil {
			return "", err
		}

		obj = nobj

		done[dep.Hash] = true
	}

	return obj, nil
}

var DiffCommand = cli.Command{
	Name:        "diff",
	Usage:       "gx diff <old> <new>",
	Description: "gx diff prints the changes between two given packages",
	Action: func(c *cli.Context) error {
		if len(c.Args()) != 2 {
			return fmt.Errorf("gx diff takes two arguments")
		}
		a := c.Args()[0]
		b := c.Args()[1]

		diff, err := DiffPackages(a, b)
		if err != nil {
			return err
		}

		diff.Print(true)
		diff.Cleanup()
		return nil
	},
}

var SetCommand = cli.Command{
	Name:  "set",
	Usage: "set package information",
	Description: `set can be used to change package information.
EXAMPLE:
   > gx set license MIT
`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "in-json",
			Usage: "Interpret input as json",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() < 2 {
			log.Fatal("must specify query and value")
		}

		var cfg map[string]interface{}
		err := gx.LoadPackageFile(&cfg, PkgFileName)
		if err != nil {
			return err
		}

		queryStr := c.Args().Get(0)
		valueStr := c.Args().Get(1)
		var value interface{} = valueStr
		if c.Bool("in-json") {
			err = json.Unmarshal([]byte(valueStr), &value)
			if err != nil {
				return err
			}
		}

		err = filter.Set(cfg, queryStr, value)
		if err != nil {
			return err
		}

		return gx.SavePackageFile(cfg, PkgFileName)
	},
}

var ReleaseCommand = cli.Command{
	Name:        "release",
	Usage:       "perform a release of a package",
	Description: `release updates the package version, publishes the package, and runs a configured release script.`,
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			log.Fatal("must specify release severity (major, minor, patch)")
		}

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		err = updateVersion(pkg, c.Args().First())
		if err != nil {
			return err
		}

		fmt.Printf("publishing package...\r")
		err = doPublish(pkg)
		if err != nil {
			return err
		}

		return runRelease(pkg)
	},
}

var TestCommand = cli.Command{
	Name:            "test",
	Usage:           "run package tests",
	Description:     `Runs a pre-test setup hook, the test command itself, and then a post-test cleanup hook.`,
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		err = gx.TryRunHook("pre-test", pkg.Language, pkg.SubtoolRequired)
		if err != nil {
			return err
		}

		var testErr error
		if pkg.Test != "" {
			testErr = fmt.Errorf("don't support running custom test script yet, bug whyrusleeping")
		} else {
			testErr = gx.TryRunHook("test", pkg.Language, pkg.SubtoolRequired, c.Args()...)
		}

		err = gx.TryRunHook("post-test", pkg.Language, pkg.SubtoolRequired)
		if err != nil {
			return err
		}

		return testErr
	},
}

func splitArgs(in string) []string {
	var out []string

	var inquotes bool
	cur := 0
	for i, c := range in {
		switch {
		case c == '"':
			if inquotes {
				out = append(out, in[cur:i])
				inquotes = false
			} else {
				inquotes = true
			}
			cur = i + 1
		case c == ' ':
			if inquotes {
				continue
			}
			if i == cur {
				cur++
				continue
			}

			out = append(out, in[cur:i])
			cur = i + 1
		}
	}

	final := in[cur:]
	if final != "" {
		out = append(out, final)
	}

	return out
}

func escapeReleaseCmd(pkg *gx.Package, cmd string) string {
	cmd = strings.Replace(cmd, "$VERSION", pkg.Version, -1)

	return cmd
}

func runRelease(pkg *gx.Package) error {
	if pkg.ReleaseCmd == "" {
		return nil
	}

	replaced := escapeReleaseCmd(pkg, pkg.ReleaseCmd)

	parts := splitArgs(replaced)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
