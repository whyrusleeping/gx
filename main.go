package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
)

var pm *gx.PM

const PkgFileName = gx.PkgFileName

func main() {
	cfg, err := gx.LoadConfig()
	if err != nil {
		Fatal(err)
	}

	pm = gx.NewPM(cfg)

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

		err = pm.CheckDaemon()
		if err != nil {
			str := fmt.Sprintf("%s requires a running ipfs daemon", os.Args[0])
			if Verbose {
				return fmt.Errorf(str+" (%s)", err)
			}
			return fmt.Errorf(str)
		}

		return nil
	}

	app.Usage = "gx is a packaging tool that uses ipfs"

	var PublishCommand = cli.Command{
		Name:  "publish",
		Usage: "publish a package",
		Action: func(c *cli.Context) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Fatal(err)
			}

			hash, err := pm.PublishPackage(cwd, pkg)
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
		},
	}

	var ImportCommand = cli.Command{
		Name:  "import",
		Usage: "import a package as a dependency",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Usage: "specify an alternative name for the package",
			},
			cli.BoolFlag{
				Name:  "nolink",
				Usage: "do not link package after importing",
			},
		},
		Action: func(c *cli.Context) {
			if len(c.Args()) == 0 {
				Fatal("import requires a package name")
			}

			name := c.String("name")
			nolink := c.Bool("nolink")

			pkg, err := gx.LoadPackageFile(PkgFileName)
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

			ndep, err := pm.ImportPackage(cwd, dephash, name, nolink)
			if err != nil {
				Fatal(err)
			}

			pkg.Dependencies = append(pkg.Dependencies, ndep)
			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Fatal("writing pkgfile: %s", err)
			}

			npkg, err := pm.GetPackage(dephash)
			if err != nil {
				Fatal("loading package after import: ", err)
			}

			switch npkg.Language {
			case "go":
				if npkg.Go != nil && npkg.Go.DvcsImport != "" {
					Log("To switch existing imports to new package, run:")
					Log("gx-go-tool update %s %s/%s", npkg.Go.DvcsImport, dephash, npkg.Name)
				}
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
			cli.BoolFlag{
				Name:  "nolink",
				Usage: "do not link installed packages",
			},
		},
		Action: func(c *cli.Context) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Fatal(err)
			}

			save := c.Bool("save")
			nolink := c.Bool("nolink")
			global := c.Bool("global")

			if len(c.Args()) == 0 {
				location := cwd + "/vendor/"
				if global {
					location = os.Getenv("GOPATH") + "/src"
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

				ndep, err := pm.ImportPackage(cwd, p, "", nolink)
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
		Action: func(c *cli.Context) {
			if !c.Args().Present() {
				Fatal("no package specified")
			}

			pkg := c.Args().First()

			_, err := pm.GetPackageLocalDaemon(pkg, cwd)
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

			fmt.Printf("initializing package %s...\n", pkgname)
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
		Description: `Update a package to a specified version
		
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

			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Fatal("error: ", err)
			}

			npkg, err := pm.GetPackage(target)
			if err != nil {
				Fatal("(getpackage) : ", err)
			}

			srcdir := path.Join(cwd, "vendor")

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

			if oldhash != "" {
				Log("now update your source with:")
				switch pkg.Language {
				case "go":
					Log("gx-go-tool update %s/%s %s/%s", olddep.Hash, oldhash, target, olddep.Name)
				default:
					Log("sed -i s/%s/%s/ ./*\n", oldhash, target)
				}
			}
		},
	}

	var UnlinkCommand = cli.Command{
		Name:  "unlink",
		Usage: "remove the named link for the given package",
		Action: func(c *cli.Context) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Fatal(err)
			}

			if !c.Args().Present() {
				Fatal("must specify name of dep to link")
			}

			dep := pkg.FindDep(c.Args().First())
			if dep == nil {
				Fatal("no such dep: %s", c.Args().First())
			}

			err = gx.RemoveLink(path.Join(cwd, "vendor"), dep.Hash, dep.Linkname)
			if err != nil {
				Fatal(err)
			}

			dep.Linkname = ""

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Fatal(err)
			}
		},
	}

	var LinkCommand = cli.Command{
		Name:  "link",
		Usage: "create a named link for the given package",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Usage: "specify the name of the link",
			},
		},
		Action: func(c *cli.Context) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Fatal(err)
			}

			if !c.Args().Present() {
				Fatal("must specify name of dep to link")
			}

			dep := pkg.FindDep(c.Args().First())
			if dep == nil {
				Fatal("no such dep: %s", c.Args().First())
			}

			// get name from flag or default
			name := c.String("name")
			if name == "" {
				name = dep.Name + "-v" + dep.Version
			}

			if dep.Linkname != "" {
				con := yesNoPrompt("package already has link, continue anyway?", true)
				if !con {
					return
				}

				err := gx.RemoveLink(path.Join(cwd, "vendor"), dep.Hash, name)
				if err != nil {
					Fatal(err)
				}
				dep.Linkname = ""

				err = gx.SavePackageFile(pkg, PkgFileName)
				if err != nil {
					Fatal(err)
				}
			}

			err = gx.TryLinkPackage(path.Join(cwd, "vendor"), dep.Hash, name)
			if err != nil {
				Fatal(err)
			}

			dep.Linkname = name

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Fatal(err)
			}
		},
	}

	var VersionCommand = cli.Command{
		Name:  "version",
		Usage: "view or modify this packages version",
		Action: func(c *cli.Context) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
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
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Fatal(err)
			}

			dry := c.Bool("dry-run")

			good := make(map[string]struct{})
			for _, dep := range pkg.Dependencies {
				good[dep.Hash] = struct{}{}
				if dep.Linkname != "" {
					good[dep.Linkname] = struct{}{}
				}
			}

			vdir := filepath.Join(cwd, "vendor")
			dirinfos, err := ioutil.ReadDir(vdir)
			if err != nil {
				Fatal(err)
			}

			for _, di := range dirinfos {
				_, keep := good[di.Name()]
				if !keep {
					fmt.Println(di.Name())
					if !dry {
						err := os.RemoveAll(filepath.Join(cwd, "vendor", di.Name()))
						if err != nil {
							Fatal(err)
						}
					}
				}
			}
		},
	}

	app.Commands = []cli.Command{
		CleanCommand,
		GetCommand,
		ImportCommand,
		InitCommand,
		InstallCommand,
		LinkCommand,
		PublishCommand,
		UnlinkCommand,
		UpdateCommand,
		VersionCommand,
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
