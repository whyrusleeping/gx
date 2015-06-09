package main

import (
	"fmt"
	"os"
	"path"
	"sort"

	sh "github.com/whyrusleeping/ipfs-shell"
)

const PkgFileName = "package.json"

type PM struct {
	shell *sh.Shell

	// hash of the 'empty' ipfs dir
	blankDir string
}

// InstallPackage recursively installs all dependencies for the given package
func (pm *PM) InstallPackage(pkg *Package) error {
	for _, dep := range pkg.Dependencies {
		pkg, err := pm.GetPackage(dep.Hash)
		if err != nil {
			return fmt.Errorf("failed to fetch package: %s (%s):%s", dep.Name, dep.Hash, err)
		}

		err = pm.InstallPackage(pkg)
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
	if len(os.Args) == 1 {
		fmt.Printf("usage: %s [command]", os.Args[0])
		return
	}

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

	switch os.Args[1] {
	case "publish":
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

	case "import":
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			fmt.Println("error: ", err)
			return
		}

		ndep, err := pm.GetPackage(os.Args[2])
		if err != nil {
			fmt.Printf("error: %s\n", err)
			return
		}

		pkg.Dependencies = append(pkg.Dependencies,
			&Dependency{
				Name: ndep.Name,
				Hash: os.Args[2],
			},
		)

		err = SavePackageFile(pkg, PkgFileName)
		if err != nil {
			fmt.Printf("error writing pkgfile: %s\n", err)
			return
		}

	case "install":
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			fmt.Println("error: ", err)
			return
		}

		err = pm.InstallPackage(pkg)
		if err != nil {
			fmt.Println(err)
			return
		}

	case "add":
		if len(os.Args) < 3 {
			fmt.Println("add requires a file name")
			return
		}
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			fmt.Println("error: ", err)
			return
		}

		set := make(map[string]struct{})
		for _, fi := range os.Args[2:] {
			set[path.Clean(fi)] = struct{}{}
		}
		for _, fi := range pkg.Files {
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
	case "rm":
		if len(os.Args) < 3 {
			fmt.Println("add requires a file name")
			return
		}
		pkg, err := LoadPackageFile(PkgFileName)
		if err != nil {
			fmt.Println("error: ", err)
			return
		}

		var out []string
		for _, fi := range pkg.Files {
			if fi != os.Args[2] {
				out = append(out, fi)
			}
		}
		pkg.Files = out
		err = SavePackageFile(pkg, PkgFileName)
		if err != nil {
			fmt.Printf("error writing package file: %s\n", err)
			return
		}

	case "get":
		if len(os.Args) < 3 {
			fmt.Println("no package specified")
			return
		}

		pkg := os.Args[2]

		_, err := pm.getPackageLocalDaemon(pkg, cwd)
		if err != nil {
			fmt.Printf("error fetching package: %s\n", err)
			return
		}
	case "init":
		var pkgname string
		if len(os.Args) > 2 {
			pkgname = os.Args[2]
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
		err = SavePackageFile(pkg, PkgFileName)
		if err != nil {
			fmt.Printf("save error: %s\n", err)
			return
		}

	default:
		fmt.Println("unrecognized command")
	}
}
