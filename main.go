package main

import (
	"fmt"
	"os"
	"path"

	"github.com/blang/semver"
	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
)

const PkgFileName = gx.PkgFileName

func main() {

	pm := gx.NewPM()

	var cwd string
	var global bool
	var lang string

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
				Error(err.Error())
				return
			}

			hash, err := pm.PublishPackage(cwd, pkg)
			if err != nil {
				Error(err.Error())
				return
			}
			Log("package %s published with hash: %s", pkg.Name, hash)

			// write out version hash
			fi, err := os.Create(".gxlastpubver")
			if err != nil {
				Error("failed to create version file: %s", err)
				return
			}

			LogV("writing published version to .gxlastpubver")
			_, err = fi.Write([]byte(hash))
			if err != nil {
				Error("failed to write version file: %s\n", err)
				return
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
		},
		Action: func(c *cli.Context) {
			if len(c.Args()) == 0 {
				Error("import requires a package name")
				return
			}

			name := c.String("name")

			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err.Error())
				return
			}

			depname := c.Args().First()

			ndep, err := pm.GetPackage(depname)
			if err != nil {
				Error(err.Error())
				return
			}

			if len(name) == 0 {
				name = ndep.Name + "-v" + ndep.Version
			}

			var linkname string

			err = gx.TryLinkPackage(path.Join(cwd, "vendor"), depname, name)
			switch err {
			case nil:
				Log("package symlinked as '%s'", name)
				linkname = name
			case gx.ErrLinkAlreadyExists:
				Log("a package with the same name already exists, skipping link step...")
			default:
				Error(err.Error())
				return
			}

			for _, cdep := range pkg.Dependencies {
				if cdep.Hash == depname {
					Error("package already imported")
					return
				}
			}

			pkg.Dependencies = append(pkg.Dependencies,
				&gx.Dependency{
					Name:     ndep.Name,
					Hash:     depname,
					Linkname: linkname,
				},
			)

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Error("writing pkgfile: %s", err)
				return
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
		},
		Action: func(c *cli.Context) {
			if len(c.Args()) > 0 {
				Error("this command currently just installs the package in your current directory")
			}
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err.Error())
				return
			}
			location := cwd + "/vendor/"
			if global {
				location = os.Getenv("GOPATH") + "/src"
			}

			err = pm.InstallDeps(pkg, location)
			if err != nil {
				Error(err.Error())
				return
			}
		},
	}

	var GetCommand = cli.Command{
		Name:  "get",
		Usage: "download a package",
		Action: func(c *cli.Context) {
			if len(c.Args()) == 0 {
				Error("no package specified")
				return
			}

			pkg := c.Args().First()

			_, err := pm.GetPackageLocalDaemon(pkg, cwd)
			if err != nil {
				Error("fetching package: %s", err)
				return
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

			fmt.Printf("initializing package %s...\n", pkgname)
			err := gx.InitPkg(cwd, pkgname, lang)
			if err != nil {
				fmt.Printf("init error: %s\n", err)
				return
			}
		},
	}

	/* Update command example
			Example: `
	  Update 'myPkg' to a given version (referencing it by package name):

	  $ gx update myPkg QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1

	  or reference it by hash:

	  $ export OLDHASH=QmdTTcAwxWhHLruoZtowxuqua1e5GVkYzxziiYPDn4vWJb
	  $ gx update $OLDHASH QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1

	  $ export OLDHASH=(readlink vendor/myPkg-v1.3.1)
	  $ gx update $OLDHASH QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1

	`,*/

	var UpdateCommand = cli.Command{
		Name:  "update",
		Usage: "update a package dependency",
		Action: func(c *cli.Context) {
			if len(c.Args()) < 2 {
				fmt.Println("update requires two arguments, current and target")
				return
			}

			existing := c.Args()[0]
			target := c.Args()[1]
			// TODO: ensure both args are the 'same' package (same name at least)

			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			npkg, err := pm.GetPackage(target)
			if err != nil {
				Error("(getpackage) : ", err)
				return
			}

			srcdir := path.Join(cwd, "vendor")

			err = pm.InstallDeps(npkg, srcdir)
			if err != nil {
				Error("(installdeps) : ", err)
				return
			}

			var oldhash string
			for _, dep := range pkg.Dependencies {
				if dep.Hash == existing || dep.Name == existing {
					oldhash = dep.Hash
					dep.Hash = target
					break
				}
			}

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Error("writing package file: %s\n", err)
				return
			}

			if oldhash != "" {
				Log("now update your source with:")
				Log("sed -i s/%s/%s/ ./*\n", oldhash, target)
			}
		},
	}

	var VersionCommand = cli.Command{
		Name:  "version",
		Usage: "view of modify this packages version",
		Action: func(c *cli.Context) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err.Error())
				return
			}

			if !c.Args().Present() {
				fmt.Println(pkg.Version)
				return
			}

			defer func() {
				err := gx.SavePackageFile(pkg, PkgFileName)
				if err != nil {
					Error(err.Error())
					return
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
				Error(err.Error())
				return
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

	app.Commands = []cli.Command{
		InitCommand,
		InstallCommand,
		UpdateCommand,
		VersionCommand,
		GetCommand,
		ImportCommand,
		PublishCommand,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
