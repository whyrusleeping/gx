package gxutil

import (
	"errors"
	"fmt"
	"os"
	"path"

	sh "github.com/ipfs/go-ipfs-api"
)

const PkgFileName = "package.json"

var ErrLinkAlreadyExists = fmt.Errorf("named package already exists")

type PM struct {
	shell *sh.Shell

	// hash of the 'empty' ipfs dir to avoid extra calls to object new
	blankDir string
}

func NewPM() *PM {
	return &PM{
		shell: sh.NewShell(getDaemonAddr()),
	}
}

// InstallDeps recursively installs all dependencies for the given package
func (pm *PM) InstallDeps(pkg *Package, location string) error {
	fmt.Printf("installing package: %s-%s\n", pkg.Name, pkg.Version)
	for _, dep := range pkg.Dependencies {

		// if its already local, skip it
		pkgdir := path.Join(location, dep.Hash)
		_, err := findPackageInDir(pkgdir)
		if err == nil {
			continue
		}

		pkg, err := pm.GetPackageLocalDaemon(dep.Hash, location)
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

func TryLinkPackage(dir, hash, name string) error {
	finfo, err := os.Lstat(path.Join(dir, name))
	if err == nil {
		if finfo.Mode()&os.ModeSymlink != 0 {
			return ErrLinkAlreadyExists
		} else {
			return fmt.Errorf("link target exists and is not a symlink")
		}
	}

	if !os.IsNotExist(err) {
		return err
	}

	pkgname, err := packageNameInDir(path.Join(dir, hash))
	if err != nil {
		return err
	}

	return os.Symlink(path.Join(hash, pkgname), path.Join(dir, name))
}

func InitPkg(dir, name, lang string) error {
	// check for existing packagefile
	p := path.Join(dir, PkgFileName)
	_, err := os.Stat(p)
	if err == nil {
		return errors.New("package file already exists in working dir")
	}

	pkg := new(Package)
	pkg.Name = name
	pkg.Language = lang
	pkg.Version = "1.0.0"
	return SavePackageFile(pkg, p)
}
