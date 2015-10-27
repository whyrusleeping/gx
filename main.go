package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/blang/semver"
	cli "github.com/codegangsta/cli"
	gx "github.com/whyrusleeping/gx/gxutil"
)

const PkgFileName = gx.PkgFileName

func main() {
	pm := gx.NewPM()

	var cwd string
	var global bool

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
				Error(err)
				return
			}

			hash, err := pm.PublishPackage(cwd, pkg)
			if err != nil {
				Error(err)
				return
			}
			Log("package %s published with hash: %s", pkg.Name, hash)

			// write out version hash
			fi, err := os.Create(".gxlastpubver")
			if err != nil {
				Error("failed to create version file: %s", err)
				return
			}

			VLog("writing published version to .gxlastpubver")
			_, err = fi.Write([]byte(hash))
			if err != nil {
				Error("failed to write version file: %s", err)
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
			cli.BoolFlag{
				Name:  "nolink",
				Usage: "do not link package after importing",
			},
		},
		Action: func(c *cli.Context) {
			if len(c.Args()) == 0 {
				Error("import requires a package name")
				return
			}

			name := c.String("name")
			nolink := c.Bool("nolink")

			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err)
				return
			}

			depname := c.Args().First()

			ndep, err := pm.GetPackage(depname)
			if err != nil {
				Error(err)
				return
			}

			if len(name) == 0 {
				name = ndep.Name + "-v" + ndep.Version
			}

			cdep := pkg.FindDep(depname)
			if cdep != nil {
				Error("package %s already imported as %s", cdep.Hash, cdep.Name)
				return
			}

			var linkname string

			if !nolink {
				err = gx.TryLinkPackage(path.Join(cwd, "vendor"), depname, name)
				switch err {
				case nil:
					Log("package symlinked as '%s'", name)
					linkname = name
				case gx.ErrLinkAlreadyExists:
					Log("a package with the same name already exists, skipping link step...")
				default:
					Error(err)
					return
				}
			}

			pkg.Dependencies = append(pkg.Dependencies,
				&gx.Dependency{
					Name:     ndep.Name,
					Hash:     depname,
					Linkname: linkname,
					Version:  ndep.Version,
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
				Error(err)
				return
			}
			location := cwd + "/vendor/"
			if global {
				location = os.Getenv("GOPATH") + "/src"
			}

			err = pm.InstallDeps(pkg, location)
			if err != nil {
				Error(err)
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

			lang := c.String("lang")
			if !c.IsSet("lang") {
				lang = promptUser("what language will the project be in?")
			}

			fmt.Printf("initializing package %s...\n", pkgname)
			err := gx.InitPkg(cwd, pkgname, lang)
			if err != nil {
				fmt.Printf("init error: %s\n", err)
				return
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
			olddep := pkg.FindDep(existing)
			if olddep != nil {
				oldhash = olddep.Hash
				olddep.Hash = target
			}

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Error("writing package file: %s", err)
				return
			}

			if oldhash != "" {
				Log("now update your source with:")
				switch pkg.Language {
				case "go":
					Log("gx-go-tool update %s/%s %s/%s", olddep.Hash, olddep.Name, target, olddep.Name)
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
				Error(err.Error())
				return
			}

			if !c.Args().Present() {
				Log("must specify name of dep to link")
				return
			}

			dep := pkg.FindDep(c.Args().First())
			if dep == nil {
				Error("no such dep: %s", c.Args().First())
				return
			}

			err = gx.RemoveLink(path.Join(cwd, "vendor"), dep.Hash, dep.Linkname)
			if err != nil {
				Error(err)
				return
			}

			dep.Linkname = ""

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Error(err)
				return
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
				Error(err.Error())
				return
			}

			if !c.Args().Present() {
				Log("must specify name of dep to link")
				return
			}

			dep := pkg.FindDep(c.Args().First())
			if dep == nil {
				Error("no such dep: %s", c.Args().First())
				return
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
					Error(err)
					return
				}
				dep.Linkname = ""

				err = gx.SavePackageFile(pkg, PkgFileName)
				if err != nil {
					Error(err)
					return
				}
			}

			err = gx.TryLinkPackage(path.Join(cwd, "vendor"), dep.Hash, name)
			if err != nil {
				Error(err)
				return
			}

			dep.Linkname = name

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Error(err)
				return
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
		LinkCommand,
		UnlinkCommand,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
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
