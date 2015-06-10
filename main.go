package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"

	cobra "QmR5FHS9TpLbL9oYY8ZDR3A7UWcHTBawU1FJ6pu9SvTcPa/cobra"
	sh "github.com/whyrusleeping/ipfs-shell"
)

const PkgFileName = "package.json"

type PM struct {
	shell *sh.Shell

	// hash of the 'empty' ipfs dir
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

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	pm := &PM{
		shell: sh.NewShell("localhost:5001"),
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

	var AddCommand = &cobra.Command{
		Use:   "add",
		Short: "add a file as a package requirement",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				fmt.Println("add requires a file name")
				return
			}

			pkg, err := LoadPackageFile(PkgFileName)
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			// ensure no duplicates
			set := make(map[string]struct{})
			for _, fi := range pkg.Files {
				set[fi] = struct{}{}
			}

			for _, fi := range args {
				fi = path.Clean(fi)
				set[fi] = struct{}{}
			}

			var out []string
			for fi, _ := range set {
				out = append(out, fi)
			}

			sort.Strings(out)

			pkg.Files = out
			err = SavePackageFile(pkg, PkgFileName)
			if err != nil {
				fmt.Printf("error writing package file: %s\n", err)
				return
			}
		},
	}

	var PublishCommand = &cobra.Command{
		Use:   "publish",
		Short: "publish a package",
		Run: func(cmd *cobra.Command, args []string) {
			pkg, err := LoadPackageFile(PkgFileName)
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			hash, err := pm.PublishPackage(cwd, pkg)
			if err != nil {
				fmt.Printf("error: %s\n", err)
				return
			}
			fmt.Printf("package %s published with hash: %s\n", pkg.Name, hash)

			// write out version hash
			fi, err := os.Create(".gxlastpubver")
			if err != nil {
				fmt.Printf("failed to create version file: %s\n", err)
				return
			}
			_, err = fi.Write([]byte(hash))
			if err != nil {
				fmt.Printf("failed to write version file: %s\n", err)
				return
			}
		},
	}

	var ImportCommand = &cobra.Command{
		Use:   "import",
		Short: "import a package as a dependency",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("import requires a package name")
				return
			}

			pkg, err := LoadPackageFile(PkgFileName)
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			depname := args[0]

			ndep, err := pm.GetPackage(depname)
			if err != nil {
				fmt.Printf("error: %s\n", err)
				return
			}

			pkg.Dependencies = append(pkg.Dependencies,
				&Dependency{
					Name: ndep.Name,
					Hash: depname,
				},
			)

			err = SavePackageFile(pkg, PkgFileName)
			if err != nil {
				fmt.Printf("error writing pkgfile: %s\n", err)
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
				fmt.Println("error: ", err)
				return
			}
			location := cwd + "/pkg/src"
			if global {
				location = os.Getenv("GOPATH") + "/src"
			}

			err = pm.InstallDeps(pkg, location)
			if err != nil {
				fmt.Println(err)
				return
			}
		},
	}

	var RmCommand = &cobra.Command{
		Use:   "rm",
		Short: "remove a file from the local package",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("rm requires a file name")
				return
			}
			pkg, err := LoadPackageFile(PkgFileName)
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			var out []string
			for _, fi := range pkg.Files {
				if fi != args[0] {
					out = append(out, fi)
				}
			}
			pkg.Files = out
			err = SavePackageFile(pkg, PkgFileName)
			if err != nil {
				fmt.Printf("error writing package file: %s\n", err)
				return
			}
		},
	}

	var GetCommand = &cobra.Command{
		Use:   "get",
		Short: "download a package",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("no package specified")
				return
			}

			pkg := args[0]

			_, err := pm.getPackageLocalDaemon(pkg, cwd)
			if err != nil {
				fmt.Printf("error fetching package: %s\n", err)
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
				fmt.Println("error: ", err)
				return
			}

			err = pm.InstallDeps(npkg, cwd+"/pkg/src")
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			for _, dep := range pkg.Dependencies {
				if dep.Hash == existing {
					dep.Hash = target
				}
			}

			err = SavePackageFile(pkg, PkgFileName)
			if err != nil {
				fmt.Printf("error writing package file: %s\n", err)
				return
			}

			fmt.Println("now update your source with:")
			fmt.Printf("sed -i s/%s/%s/ ./*\n", existing, target)
		},
	}

	var BuildCommand = &cobra.Command{
		Use:   "build",
		Short: "build a package",
		Run: func(cmd *cobra.Command, args []string) {
			pkg, err := LoadPackageFile(PkgFileName)
			if err != nil {
				fmt.Println("error: ", err)
				return
			}

			switch pkg.Language {
			case "go":
				env := os.Getenv("GOPATH")
				os.Setenv("GOPATH", env+":"+cwd+"/pkg")
				cmd := exec.Command("go", "build")
				cmd.Env = os.Environ()
				out, err := cmd.CombinedOutput()
				if err != nil {
					fmt.Printf("build error: %s\n", err)
				}
				fmt.Print(string(out))
			default:
				fmt.Println("language unrecognized or unspecified")
				return
			}
		},
	}

	GxCommand.AddCommand(AddCommand)
	GxCommand.AddCommand(PublishCommand)
	GxCommand.AddCommand(GetCommand)
	GxCommand.AddCommand(RmCommand)
	GxCommand.AddCommand(InitCommand)
	InitCommand.Flags().StringVar(&lang, "lang", "", "specify the primary language of the package")

	GxCommand.AddCommand(ImportCommand)
	GxCommand.AddCommand(InstallCommand)
	InstallCommand.Flags().BoolVar(&global, "global", false, "install to global scope")

	GxCommand.AddCommand(BuildCommand)
	GxCommand.AddCommand(UpdateCommand)
	err = GxCommand.Execute()
	if err != nil {
		fmt.Println(err)
	}
}
