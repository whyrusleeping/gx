package gxutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

func (pm *PM) GetPackage(hash, dir string) (*Package, error) {
	var pkg Package

	pkgdir := filepath.Join(dir, "vendor", hash)
	_, err := os.Stat(pkgdir)
	if err == nil {
		err := FindPackageInDir(&pkg, pkgdir)
		if err == nil {
			return &pkg, nil
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	err = pm.shell.Get(hash, pkgdir)
	if err != nil {
		return nil, err
	}

	err = FindPackageInDir(&pkg, pkgdir)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

func FindPackageInDir(pkg interface{}, dir string) error {
	name, err := PackageNameInDir(dir)
	if err != nil {
		return err
	}
	return LoadPackageFile(pkg, filepath.Join(dir, name, PkgFileName))
}

func PackageNameInDir(dir string) (string, error) {
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
