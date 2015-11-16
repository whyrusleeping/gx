package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/blang/semver"
	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
	. "github.com/whyrusleeping/stump"
)

const vendorDir = "vendor"

var pm *gx.PM

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

	var cwd string

	app := cli.NewApp()
	app.Author = "whyrusleeping"
	app.Version = "0.1"
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
				Fatal(err)
			}
			Log("package %s published with hash: %s", pkg.Name, hash)

			// write out version hash
			fi, err := os.Create(".gxlastpubver")
			if err != nil {
				Fatal("failed to create version file: %s", err)
			}

			defer fi.Close()

			VLog("writing published version to .gxlastpubver")
			_, err = fmt.Fprintf(fi, "%s: %s\n", pkg.Version, hash)
			if err != nil {
				Fatal("failed to write version file: %s", err)
			}

			err = gx.TryRunHook("post-publish", pkg.Language, hash)
			if err != nil {
				Fatal(err)
			}
		},
	}

	var ImportCommand = cli.Command{
		Name:  "import",
		Usage: "import a package as a dependency",
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

			ndep, err := pm.ImportPackage(cwd, dephash)
			if err != nil {
				Fatal(err)
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
				location := filepath.Join(cwd, vendorDir)
				if global {
					location = filepath.Join(os.Getenv("GOPATH"), "src")
				}

				err = pm.InstallDeps(pkg, location)
				if err != nil {
					Fatal(err)
				}
				return
			}

			for _, p := range c.Args() {
				phash, err := pm.ResolveDepName(p)
				if err != nil {
					Error("resolving package '%s': %s", p, err)
				}

				if p != phash {
					VLog("%s resolved to %s", p, phash)
				}

				ndep, err := pm.ImportPackage(cwd, p)
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
				out = cwd
			}

			_, err := pm.GetPackageLocalDaemon(pkg, out)
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
			err := pm.InitPkg(cwd, pkgname, lang)
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

			npkg, err := pm.GetPackage(target)
			if err != nil {
				Fatal("(getpackage) : ", err)
			}

			srcdir := filepath.Join(cwd, vendorDir)

			err = pm.InstallDeps(npkg, srcdir)
			if err != nil {
				Fatal("(installdeps) : ", err)
			}

			var oldhash string
			olddep := pkg.FindDep(existing)
			if olddep != nil {
				oldhash = olddep.Hash
				olddep.Hash = target
			}

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Fatal("writing package file: %s", err)
			}

			err = gx.TryRunHook("post-update", pkg.Language, oldhash, target)
			if err != nil {
				Fatal(err)
			}
		},
	}

	var VersionCommand = cli.Command{
		Name:  "version",
		Usage: "view or modify this packages version",
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
		Action: func(c *cli.Context) {
			if !c.Args().Present() {
				Fatal("must specify at least a query")
			}
			fname := PkgFileName
			queryStr := c.Args()[0]
			if len(c.Args()) == 2 {
				pkgdir := filepath.Join(vendorDir, c.Args()[0])
				name, err := gx.PackageNameInDir(pkgdir)
				if err != nil {
					Fatal(err)
				}

				fname = filepath.Join(pkgdir, name, PkgFileName)
				queryStr = c.Args()[1]
			}

			fi, err := os.Open(fname)
			if err != nil {
				Fatal(err)
			}

			var cfg map[string]interface{}
			err = json.NewDecoder(fi).Decode(&cfg)
			if err != nil {
				Fatal(err)
			}
			fi.Close()

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
					dpkg, err := pm.GetPackage(d)
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

func promptUser(query string) string {
	fmt.Printf("%s ", query)
	scan := bufio.NewScanner(os.Stdin)
	scan.Scan()
	return scan.Text()
}

func yesNoPrompt(prompt string, def bool) bool {
	opts := "[y/N]"
	if def {
		opts = "[Y/n]"
	}

	fmt.Printf("%s %s ", prompt, opts)
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		val := strings.ToLower(scan.Text())
		switch val {
		case "":
			return def
		case "y":
			return true
		case "n":
			return false
		default:
			fmt.Println("please type 'y' or 'n'")
		}
	}

	panic("unexpected termination of stdin")
}

func jsonPrint(i interface{}) {
	out, _ := json.MarshalIndent(i, "", "  ")
	outs, err := strconv.Unquote(string(out))
	if err != nil {
		outs = string(out)
	}
	Log(outs)
}

func printDepsTree(pm *gx.PM, pkg *gx.Package, quiet bool, indent int) error {
	for _, d := range pkg.Dependencies {
		label := d.Hash
		if !quiet {
			label = fmt.Sprintf("%s %s %s", d.Name, d.Hash, d.Version)
		}
		Log("%s%s", strings.Repeat("  ", indent), label)
		npkg, err := pm.GetPackage(d.Hash)
		if err != nil {
			return err
		}

		err = printDepsTree(pm, npkg, quiet, indent+1)
		if err != nil {
			return err
		}
	}
	return nil
}
