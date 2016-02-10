package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/blang/semver"
	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
	. "github.com/whyrusleeping/stump"
)

var (
	vendorDir = filepath.Join("vendor", "gx", "ipfs")
	cwd       string
	pm        *gx.PM
)

const PkgFileName = gx.PkgFileName

func LoadPackageFile(path string) (*gx.Package, error) {
	var pkg gx.Package
	err := gx.LoadPackageFile(&pkg, path)
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func main() {
	cfg, err := gx.LoadConfig()
	if err != nil {
		Fatal(err)
	}

	pm, err = gx.NewPM(cfg)
	if err != nil {
		Fatal(err)
	}

	app := cli.NewApp()
	app.Author = "whyrusleeping"
	app.Version = "0.3"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "print verbose logging information",
		},
	}
	app.Before = func(c *cli.Context) error {
		Verbose = c.Bool("verbose")

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
		InitCommand,
		InstallCommand,
		PublishCommand,
		RepoCommand,
		UpdateCommand,
		VersionCommand,
		ViewCommand,
	}

	app.RunAndExitOnError()
}

var PublishCommand = cli.Command{
	Name:  "publish",
	Usage: "publish a package",
	Action: func(c *cli.Context) {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal(err)
		}

		err = gx.TryRunHook("pre-publish", pkg.Language)
		if err != nil {
			Fatal(err)
		}

		hash, err := pm.PublishPackage(cwd, &pkg.PackageBase)
		if err != nil {
			Fatal("publishing: ", err)
		}
		Log("package %s published with hash: %s", pkg.Name, hash)

		// write out version hash
		err = writeLastPub(pkg.Version, hash)
		if err != nil {
			Fatal(err)
		}

		err = gx.TryRunHook("post-publish", pkg.Language, hash)
		if err != nil {
			Fatal(err)
		}
	},
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

	VLog("writing published version to .gx/lastpubver")
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
		cli.BoolFlag{
			Name:  "global",
			Usage: "download imported package to global store",
		},
	},
	Action: func(c *cli.Context) {
		if len(c.Args()) == 0 {
			Fatal("import requires a package name")
		}

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal(err)
		}

		depname := c.Args().First()
		cdep := pkg.FindDep(depname)
		if cdep != nil {
			Fatal("package %s already imported as %s", cdep.Hash, cdep.Name)
		}

		dephash, err := pm.ResolveDepName(depname)
		if err != nil {
			Fatal(err)
		}

		ipath, err := gx.InstallPath(pkg.Language, "", c.Bool("global"))
		if err != nil {
			Fatal(err)
		}

		ndep, err := pm.ImportPackage(ipath, dephash)
		if err != nil {
			Fatal("(import):", err)
		}

		if pkg.FindDep(ndep.Name) != nil {
			s := fmt.Sprintf("package with name %s already imported, continue?", ndep.Name)
			if !yesNoPrompt(s, false) {
				return
			}
			Log("continuing, please note some things may not work as expected")
		}

		pkg.Dependencies = append(pkg.Dependencies, ndep)
		err = gx.SavePackageFile(pkg, PkgFileName)
		if err != nil {
			Fatal("writing pkgfile: %s", err)
		}

	},
}

var InstallCommand = cli.Command{
	Name:    "install",
	Usage:   "install this package",
	Aliases: []string{"i"},
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "global",
			Usage: "install package in global namespace",
		},
		cli.BoolFlag{
			Name:  "save",
			Usage: "write installed packages as deps in package.json",
		},
	},
	Action: func(c *cli.Context) {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal(err)
		}

		save := c.Bool("save")
		global := c.Bool("global")

		if len(c.Args()) == 0 {
			cwd, err := os.Getwd()
			if err != nil {
				Fatal(err)
			}

			ipath, err := gx.InstallPath(pkg.Language, cwd, global)
			if err != nil {
				Fatal(err)
			}

			err = pm.InstallDeps(pkg, ipath)
			if err != nil {
				Fatal("install deps:", err)
			}
			return
		}

		ipath, err := gx.InstallPath(pkg.Language, "", c.Bool("global"))
		if err != nil {
			Fatal(err)
		}

		for _, p := range c.Args() {
			phash, err := pm.ResolveDepName(p)
			if err != nil {
				Error("resolving package '%s': %s", p, err)
			}

			if p != phash {
				VLog("%s resolved to %s", p, phash)
			}

			ndep, err := pm.ImportPackage(ipath, p)
			if err != nil {
				Fatal("importing package '%s': %s", p, err)
			}

			if save {
				pkg.Dependencies = append(pkg.Dependencies, ndep)
			}
		}

		if save {
			err := gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Fatal(err)
			}
		}
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
	Action: func(c *cli.Context) {
		if !c.Args().Present() {
			Fatal("no package specified")
		}

		pkg := c.Args().First()

		out := c.String("o")
		if out == "" {
			out = filepath.Join(cwd, pkg)
		}

		Log("writing package to:", out)
		_, err := pm.GetPackageTo(pkg, out)
		if err != nil {
			Fatal("fetching package: %s", err)
		}
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
	Action: func(c *cli.Context) {
		var pkgname string
		if len(c.Args()) > 0 {
			pkgname = c.Args().First()
		} else {
			pkgname = path.Base(cwd)
		}

		lang := c.String("lang")
		if !c.IsSet("lang") {
			lang = promptUser("what language will the project be in?")
		}

		Log("initializing package %s...", pkgname)
		err := pm.InitPkg(cwd, pkgname, lang, func(p *gx.Package) {
			p.Issues = promptUser("where should users go to report issues?")
		})

		if err != nil {
			Fatal("init error: %s", err)
		}
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
		cli.BoolFlag{
			Name:  "global",
			Usage: "install new package in global namespace",
		},
	},
	Action: func(c *cli.Context) {
		if len(c.Args()) < 2 {
			Fatal("update requires two arguments, current and target")
		}

		existing := c.Args()[0]
		target := c.Args()[1]
		// TODO: ensure both args are the 'same' package (same name at least)

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal("error: ", err)
		}

		var oldhash string
		olddep := pkg.FindDep(existing)
		if olddep == nil {
			Fatal("unknown package: ", existing)
		}

		ipath, err := gx.InstallPath(pkg.Language, cwd, c.Bool("global"))
		if err != nil {
			Fatal(err)
		}

		npkg, err := pm.InstallPackage(target, ipath)
		if err != nil {
			Fatal("(installpackage) : ", err)
		}

		if npkg.Name != olddep.Name {
			prompt := fmt.Sprintf(`Target package has a different name than new package:
old: %s (%s)
new: %s (%s)
continue?`, olddep.Name, olddep.Hash, npkg.Name, target)
			if !yesNoPrompt(prompt, false) {
				Fatal("refusing to update package with different names")
			}
		}

		VLog("checking for potential package naming collisions...")
		err = updateCollisionCheck(pkg, olddep, nil)
		if err != nil {
			Fatal("update sanity check: ", err)
		}
		VLog("  - no collisions found for updated package")

		oldhash = olddep.Hash
		olddep.Hash = target

		err = gx.SavePackageFile(pkg, PkgFileName)
		if err != nil {
			Fatal("writing package file: %s", err)
		}

		VLog("running post update hook...")
		err = gx.TryRunHook("post-update", pkg.Language, oldhash, target)
		if err != nil {
			Fatal(err)
		}

		VLog("update complete!")
	},
}

func updateCollisionCheck(ipkg *gx.Package, idep *gx.Dependency, chain []string) error {
	return ipkg.ForEachDep(func(dep *gx.Dependency, pkg *gx.Package) error {
		if dep == idep {
			return nil
		}

		if dep.Name == idep.Name || dep.Hash == idep.Hash {
			Log("dep %s also imports %s (%s)", strings.Join(chain, "/"), dep.Name, dep.Hash)
			return nil
		}

		return updateCollisionCheck(pkg, idep, append(chain, dep.Name))
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
   by one.`,
	Action: func(c *cli.Context) {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal(err)
		}

		if !c.Args().Present() {
			fmt.Println(pkg.Version)
			return
		}

		defer func() {
			err := gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Fatal(err)
			}
		}()

		nver := c.Args().First()
		// if argument is a semver, set version to it
		_, err = semver.Make(nver)
		if err == nil {
			pkg.Version = nver
			return
		}

		v, err := semver.Make(pkg.Version)
		if err != nil {
			Fatal(err)
		}
		switch nver {
		case "major":
			v.Major++
			v.Minor = 0
			v.Patch = 0
		case "minor":
			v.Minor++
			v.Patch = 0
		case "patch":
			v.Patch++
		default:
			Error("argument was not a semver field: '%s'", nver)
			return
		}

		pkg.Version = v.String()
	},
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
`,
	Action: func(c *cli.Context) {
		if !c.Args().Present() {
			Fatal("must specify at least a query")
		}

		vendir := filepath.Join(cwd, vendorDir)

		var cfg map[string]interface{}
		if len(c.Args()) == 2 {
			ref := c.Args()[0]
			err := gx.LocalPackageByName(vendir, ref, &cfg)
			if err != nil {
				Fatal(err)
			}
		} else {
			err := gx.LoadPackageFile(&cfg, PkgFileName)
			if err != nil {
				Fatal(err)
			}
		}

		queryStr := c.Args()[len(c.Args())-1]
		var query []string
		for _, s := range strings.Split(queryStr, ".") {
			if s != "" {
				query = append(query, s)
			}
		}

		cur := cfg
		var val interface{} = cur
		for i, q := range query {
			v, ok := cur[q]
			if !ok {
				Fatal("key not found: %s", strings.Join(query[:i+1], "."))
			}
			val = v

			mp, ok := v.(map[string]interface{})
			if !ok {
				if i == len(query)-1 {
					break
				}
				Fatal("%s is not indexable", query[i-1])
			}
			cur = mp
		}

		jsonPrint(val)
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
	Action: func(c *cli.Context) {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal(err)
		}

		dry := c.Bool("dry-run")

		good, err := pm.EnumerateDependencies(pkg)
		if err != nil {
			Fatal(err)
		}

		vdir := filepath.Join(cwd, vendorDir)
		dirinfos, err := ioutil.ReadDir(vdir)
		if err != nil {
			Fatal(err)
		}

		for _, di := range dirinfos {
			if !strings.HasPrefix(di.Name(), "Qm") {
				continue
			}
			_, keep := good[di.Name()]
			if !keep {
				fmt.Println(di.Name())
				if !dry {
					err := os.RemoveAll(filepath.Join(cwd, vendorDir, di.Name()))
					if err != nil {
						Fatal(err)
					}
				}
			}
		}
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
	},
	Subcommands: []cli.Command{
		depBundleCommand,
	},
	Action: func(c *cli.Context) {
		rec := c.Bool("r")
		quiet := c.Bool("q")

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal(err)
		}

		if c.Bool("tree") {
			err := printDepsTree(pm, pkg, quiet, 0)
			if err != nil {
				Fatal(err)
			}
			return
		}

		var deps []string
		if rec {
			depmap, err := pm.EnumerateDependencies(pkg)
			if err != nil {
				Fatal(err)
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

		w := tabwriter.NewWriter(os.Stdout, 12, 4, 1, ' ', 0)
		for _, d := range deps {
			if !quiet {
				var dpkg gx.Package
				err := gx.LoadPackage(&dpkg, pkg.Language, d)
				if err != nil {
					Fatal(err)
				}

				fmt.Fprintf(w, "%s\t%s\t%s\n", dpkg.Name, d, dpkg.Version)
			} else {
				Log(d)
			}
		}
		w.Flush()
	},
}

var depBundleCommand = cli.Command{
	Name:  "bundle",
	Usage: "print hash of object containing all dependencies for this package",
	Action: func(c *cli.Context) {
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			Fatal(err)
		}

		obj, err := depBundleForPkg(pkg)
		if err != nil {
			Fatal(err)
		}

		fmt.Println(obj)
	},
}

func depBundleForPkg(pkg *gx.Package) (string, error) {
	obj, err := pm.Shell().NewObject("unixfs-dir")
	if err != nil {
		Fatal(err)
	}

	for _, dep := range pkg.Dependencies {
		Log("processing dep: ", dep.Name)
		nobj, err := pm.Shell().PatchLink(obj, dep.Name+"-"+dep.Hash, dep.Hash, false)
		if err != nil {
			return "", err
		}

		var cpkg gx.Package
		err = gx.LoadPackage(&cpkg, pkg.Language, dep.Hash)
		if err != nil {
			return "", err
		}

		child, err := depBundleForPkg(&cpkg)
		if err != nil {
			return "", err
		}

		nobj, err = pm.Shell().PatchLink(nobj, dep.Name+"-"+dep.Hash+"-deps", child, false)
		if err != nil {
			return "", err
		}

		obj = nobj
	}

	return obj, nil
}
