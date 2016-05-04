package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/blang/semver"
	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
	log "github.com/whyrusleeping/stump"
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

	if pkg.GxVersion == "" {
		pkg.GxVersion = gx.GxVersion
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
		InitCommand,
		InstallCommand,
		PublishCommand,
		RepoCommand,
		UpdateCommand,
		VersionCommand,
		ViewCommand,
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

		err = gx.TryRunHook("pre-publish", pkg.Language)
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

		err = gx.TryRunHook("post-publish", pkg.Language, hash)
		if err != nil {
			return err
		}

		return nil
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

		err = gx.TryRunHook("post-import", npkg.Language, dephash)
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

			err = gx.TryRunHook("req-check", pkg.Language, cwd)
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

		pkg := c.Args().First()

		out := c.String("o")
		if out == "" {
			out = filepath.Join(cwd, pkg)
		}

		log.Log("writing package to:", out)
		_, err := pm.GetPackageTo(pkg, out)
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
		if len(c.Args()) < 2 {
			log.Fatal("update requires two arguments, current and target")
		}

		existing := c.Args()[0]
		target := c.Args()[1]
		// TODO: ensure both args are the 'same' package (same name at least)

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			log.Fatal("error: ", err)
		}

		var oldhash string
		olddep := pkg.FindDep(existing)
		if olddep == nil {
			log.Fatal("unknown package: ", existing)
		}
		oldhash = olddep.Hash

		global := c.BoolT("global")
		if c.Bool("local") {
			global = false
		}

		ipath, err := gx.InstallPath(pkg.Language, cwd, global)
		if err != nil {
			return err
		}

		trgthash, err := pm.ResolveDepName(target)
		if err != nil {
			return err
		}

		npkg, err := pm.InstallPackage(trgthash, ipath)
		if err != nil {
			log.Fatal("(installpackage) : ", err)
		}

		if npkg.Name != olddep.Name {
			prompt := fmt.Sprintf(`Target package has a different name than new package:
old: %s (%s)
new: %s (%s)
continue?`, olddep.Name, olddep.Hash, npkg.Name, target)
			if !yesNoPrompt(prompt, false) {
				log.Fatal("refusing to update package with different names")
			}
		}

		log.VLog("running pre update hook...")
		err = gx.TryRunHook("pre-update", pkg.Language, existing)
		if err != nil {
			return err
		}

		if c.Bool("with-deps") {
			err := RecursiveDepUpdate(pkg, oldhash, target)
			if err != nil {
				return err
			}
		} else {
			log.VLog("checking for potential package naming collisions...")
			err = updateCollisionCheck(pkg, olddep, nil)
			if err != nil {
				log.Fatal("update sanity check: ", err)
			}
			log.VLog("  - no collisions found for updated package")
		}

		olddep.Hash = target
		olddep.Version = npkg.Version

		err = gx.SavePackageFile(pkg, PkgFileName)
		if err != nil {
			return fmt.Errorf("writing package file: %s", err)
		}

		log.VLog("running post update hook...")
		err = gx.TryRunHook("post-update", pkg.Language, oldhash, target)
		if err != nil {
			return err
		}

		log.VLog("update complete!")

		return nil
	},
}

func updateCollisionCheck(ipkg *gx.Package, idep *gx.Dependency, chain []string) error {
	return ipkg.ForEachDep(func(dep *gx.Dependency, pkg *gx.Package) error {
		if dep == idep {
			return nil
		}

		if dep.Name == idep.Name || dep.Hash == idep.Hash {
			log.Log("dep %s also imports %s (%s)", strings.Join(chain, "/"), dep.Name, dep.Hash)
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
			return nil
		}

		defer func() {
			err := gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				outerr = err
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
			return err
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
	Action: func(c *cli.Context) error {
		if !c.Args().Present() {
			log.Fatal("must specify at least a query")
		}

		vendir := filepath.Join(cwd, vendorDir)

		var cfg map[string]interface{}
		if len(c.Args()) == 2 {
			ref := c.Args()[0]
			err := gx.LocalPackageByName(vendir, ref, &cfg)
			if err != nil {
				return err
			}
		} else {
			err := gx.LoadPackageFile(&cfg, PkgFileName)
			if err != nil {
				return err
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
				log.Fatal("key not found: %s", strings.Join(query[:i+1], "."))
			}
			val = v

			mp, ok := v.(map[string]interface{})
			if !ok {
				if i == len(query)-1 {
					break
				}
				log.Fatal("%s is not indexable", query[i-1])
			}
			cur = mp
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

		vdir := filepath.Join(cwd, vendorDir)
		dirinfos, err := ioutil.ReadDir(vdir)
		if err != nil {
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
					err := os.RemoveAll(filepath.Join(cwd, vendorDir, di.Name()))
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
		cli.BoolFlag{
			Name:  "s,sort",
			Usage: "sort output by package name",
		},
	},
	Subcommands: []cli.Command{
		depBundleCommand,
		depFindCommand,
	},
	Action: func(c *cli.Context) error {
		rec := c.Bool("r")
		quiet := c.Bool("q")

		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			return err
		}

		if c.Bool("tree") {
			err := printDepsTree(pm, pkg, quiet, 0)
			if err != nil {
				return err
			}
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
	obj, err := pm.Shell().NewObject("unixfs-dir")
	if err != nil {
		return "", err
	}

	for _, dep := range pkg.Dependencies {
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
