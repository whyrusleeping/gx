package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type ErrAlreadyInstalled struct {
	pkg string
}

func IsErrAlreadyInstalled(err error) bool {
	_, ok := err.(ErrAlreadyInstalled)
	return ok
}

func (eai ErrAlreadyInstalled) Error() string {
	return fmt.Sprintf("package %s already installed", eai.pkg)
}

func (pm *PM) GetPackage(hash string) (*Package, error) {
	// TODO: support using gateways for package fetching
	// TODO: download packages into global package store
	//       and create readonly symlink to them in local dir
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return pm.getPackageLocalDaemon(hash, path.Join(dir, "vendor", "src"))
}

// retreive the given package from the local ipfs daemon
func (pm *PM) getPackageLocalDaemon(hash, target string) (*Package, error) {
	pkgdir := path.Join(target, hash)
	_, err := os.Stat(pkgdir)
	if err == nil {
		return findPackageInDir(pkgdir)
	}

	if !os.IsNotExist(err) {
		return nil, err
	}

	err = pm.shell.Get(hash, pkgdir)
	if err != nil {
		return nil, err
	}

	return findPackageInDir(pkgdir)
}

func findPackageInDir(dir string) (*Package, error) {
	name, err := packageNameInDir(dir)
	if err != nil {
		return nil, err
	}
	return LoadPackageFile(path.Join(dir, name, PkgFileName))
}

func packageNameInDir(dir string) (string, error) {
	fs, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	if len(fs) == 0 {
		return "", fmt.Errorf("no package found in hashdir: %s", dir)
	}

	if len(fs) > 1 {
		return "", fmt.Errorf("found multiple packages in hashdir: %s", dir)
	}

	return fs[0].Name(), nil
}
