package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	cobra "QmR5FHS9TpLbL9oYY8ZDR3A7UWcHTBawU1FJ6pu9SvTcPa/cobra"
	sh "github.com/whyrusleeping/ipfs-shell"
)

const PkgFileName = "package.json"

type PM struct {
	shell *sh.Shell

	// hash of the 'empty' ipfs dir to avoid extra calls to object new
	blankDir string
}

// InstallDeps recursively installs all dependencies for the given package
func (pm *PM) InstallDeps(pkg *Package, location string) error {
	for _, dep := range pkg.Dependencies {
		pkg, err := pm.getPackageLocalDaemon(dep.Hash, location)
		if err != nil {
			return fmt.Errorf("failed to fetch package: %s (%s):%s", dep.Name,
				dep.Hash, err)
		}

		err = pm.InstallDeps(pkg, location)
		if err != nil {
			return err
		}
	}
	return nil
}

func (pm *PM) CheckDaemon() error {
	_, err := pm.shell.ID()
	return err
}

func getDaemonAddr() string {
	da := os.Getenv("GX_IPFS_ADDR")
	if len(da) == 0 {
		return "localhost:5001"
	}
	return da
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	pm := &PM{
		shell: sh.NewShell(getDaemonAddr()),
	}

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
			pkg, err := LoadPackageFile(PkgFileName)
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
		Use:   "import",
		Short: "import a package as a dependency",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				Error("import requires a package name")
				return
			}

			name := cmd.Flag("name").Value.String()

			pkg, err := LoadPackageFile(PkgFileName)
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
				name = ndep.Name
			}

			for _, cdep := range pkg.Dependencies {
				if cdep.Hash == depname {
					Error("package already imported")
					return
				}
			}
			pkg.Dependencies = append(pkg.Dependencies,
				&Dependency{
					Name: ndep.Name,
					Hash: depname,
				},
			)

			err = SavePackageFile(pkg, PkgFileName)
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
			pkg, err := LoadPackageFile(PkgFileName)
			if err != nil {
				Error(err.Error())
				return
			}
			location := cwd + "/vendor/src"
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
		Use:   "get",
		Short: "download a package",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				Error("no package specified")
				return
			}

			pkg := args[0]

			_, err := pm.getPackageLocalDaemon(pkg, cwd)
			if err != nil {
				Error("fetching package: %s", err)
				return
			}
		},
	}

	var InitCommand = &cobra.Command{
		Use:   "init",
		Short: "initialize a package in the current working directory",
		Run: func(cmd *cobra.Command, args []string) {
			var pkgname string
			if len(args) > 0 {
				pkgname = args[0]
			} else {
				pkgname = path.Base(cwd)
			}
			fmt.Printf("initializing package %s...\n", pkgname)

			// check for existing packagefile
			_, err := os.Stat(PkgFileName)
			if err == nil {
				fmt.Println("package file already exists in working dir")
				return
			}

			pkg := new(Package)
			pkg.Name = pkgname
			pkg.Language = lang
			pkg.Version = "1.0.0"
			err = SavePackageFile(pkg, PkgFileName)
			if err != nil {
				fmt.Printf("save error: %s\n", err)
				return
			}
		},
	}

	var UpdateCommand = &cobra.Command{
		Use:   "update",
		Short: "update a package dependency",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 2 {
				fmt.Println("update requires two arguments, current and target")
				return
			}

			existing := args[0]
			target := args[1]
			// TODO: ensure both args are the 'same' package (same name at least)

			pkg, err := LoadPackageFile(PkgFileName)
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			npkg, err := pm.GetPackage(target)
			if err != nil {
				Error("(getpackage) : ", err)
				return
			}

			err = pm.InstallDeps(npkg, cwd+"/vendor/src")
			if err != nil {
				Error("(installdeps) : ", err)
				return
			}

			for _, dep := range pkg.Dependencies {
				if dep.Hash == existing {
					dep.Hash = target
				}
			}

			err = SavePackageFile(pkg, PkgFileName)
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
			pkg, err := LoadPackageFile(PkgFileName)
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
				}
				fmt.Print(string(out))
			default:
				Error("language unrecognized or unspecified")
				return
			}
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
	GxCommand.AddCommand(UpdateCommand)
	err = GxCommand.Execute()
	if err != nil {
		Error(err.Error())
	}
}
