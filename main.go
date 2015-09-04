package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/blang/semver"
	cobra "github.com/spf13/cobra"
	gx "github.com/whyrusleeping/gx/gxutil"
)

const PkgFileName = gx.PkgFileName

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	pm := gx.NewPM()

	err = pm.CheckDaemon()
	if err != nil {
		fmt.Printf("%s requires a running ipfs daemon (%s)\n", os.Args[0], err)
		return
	}

	var global bool
	var lang string

	var GxCommand = &cobra.Command{
		Use:   "gx",
		Short: "gx is a packaging tool that uses ipfs",
	}

	var PublishCommand = &cobra.Command{
		Use:   "publish",
		Short: "publish a package",
		Run: func(cmd *cobra.Command, args []string) {
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

	var ImportCommand = &cobra.Command{
		Use:   "import <pkgref>",
		Short: "import a package as a dependency",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				Error("import requires a package name")
				return
			}

			name := cmd.Flag("name").Value.String()

			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err.Error())
				return
			}

			depname := args[0]

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

	var InstallCommand = &cobra.Command{
		Use:   "install",
		Short: "install a package",
		Run: func(cmd *cobra.Command, args []string) {
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

	var GetCommand = &cobra.Command{
		Use:   "get <pkgref>",
		Short: "download a package",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				Error("no package specified")
				return
			}

			pkg := args[0]

			_, err := pm.GetPackageLocalDaemon(pkg, cwd)
			if err != nil {
				Error("fetching package: %s", err)
				return
			}
		},
	}

	var InitCommand = &cobra.Command{
		Use:   "init",
		Short: "initialize a package in the current working directory",
		Example: `
Initialize a basic package:

  $ gx init

Set the language:

  $ gx init --lang=go

`,
		Run: func(cmd *cobra.Command, args []string) {
			var pkgname string
			if len(args) > 0 {
				pkgname = args[0]
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

	var UpdateCommand = &cobra.Command{
		Use:   "update <oldref> <newref>",
		Short: "update a package dependency",
		Example: `
  Update 'myPkg' to a given version (referencing it by package name):

  $ gx update myPkg QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1

  or reference it by hash:

  $ export OLDHASH=QmdTTcAwxWhHLruoZtowxuqua1e5GVkYzxziiYPDn4vWJb 
  $ gx update $OLDHASH QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1

  $ export OLDHASH=(readlink vendor/myPkg-v1.3.1)
  $ gx update $OLDHASH QmPZ6gM12JxshKzwSyrhbEmyrsi7UaMrnoQZL6mdrzSfh1

`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 2 {
				fmt.Println("update requires two arguments, current and target")
				return
			}

			existing := args[0]
			target := args[1]
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

			for _, dep := range pkg.Dependencies {
				if dep.Hash == existing || dep.Name == existing {
					dep.Hash = target
				}
			}

			err = gx.SavePackageFile(pkg, PkgFileName)
			if err != nil {
				Error("writing package file: %s\n", err)
				return
			}

			Log("now update your source with:")
			Log("sed -i s/%s/%s/ ./*\n", existing, target)
		},
	}

	var BuildCommand = &cobra.Command{
		Use:   "build",
		Short: "build a package",
		Run: func(cmd *cobra.Command, args []string) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err.Error())
				return
			}

			switch pkg.Language {
			case "go":
				env := os.Getenv("GOPATH")
				os.Setenv("GOPATH", env+":"+cwd+"/vendor")
				cmd := exec.Command("go", "build")
				cmd.Env = os.Environ()
				out, err := cmd.CombinedOutput()
				if err != nil {
					Error("build: %s", err)
					return
				}
				fmt.Print(string(out))
			default:
				Error("language unrecognized or unspecified")
				return
			}
		},
	}

	var VersionCommand = &cobra.Command{
		Use:   "version",
		Short: "view of modify this packages version",
		Run: func(cmd *cobra.Command, args []string) {
			pkg, err := gx.LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err.Error())
				return
			}

			if len(args) == 0 {
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

			// if argument is a semver, set version to it
			_, err = semver.Make(args[0])
			if err == nil {
				pkg.Version = args[0]
				return
			}

			v, err := semver.Make(pkg.Version)
			if err != nil {
				Error(err.Error())
				return
			}
			switch args[0] {
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
				Error("argument was not a semver field: '%s'", args[0])
				return
			}

			pkg.Version = v.String()
		},
	}

	GxCommand.Flags().BoolVar(&Verbose, "v", false, "verbose output")

	GxCommand.AddCommand(PublishCommand)
	GxCommand.AddCommand(GetCommand)
	GxCommand.AddCommand(InitCommand)
	InitCommand.Flags().StringVar(&lang, "lang", "", "specify the primary language of the package")

	GxCommand.AddCommand(ImportCommand)
	ImportCommand.Flags().String("name", "", "specify the name to be used for the imported package")

	GxCommand.AddCommand(InstallCommand)
	InstallCommand.Flags().BoolVar(&global, "global", false, "install to global scope")

	GxCommand.AddCommand(BuildCommand)
	GxCommand.AddCommand(VersionCommand)
	GxCommand.AddCommand(UpdateCommand)
	err = GxCommand.Execute()
	if err != nil {
		Error(err.Error())
	}
}
