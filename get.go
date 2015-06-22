package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

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
		return nil, fmt.Errorf("package %s already installed", hash)
	}

	err = pm.shell.Get(hash, pkgdir)
	if err != nil {
		return nil, err
	}

	return findPackageInDir(pkgdir)
}

func findPackageInDir(dir string) (*Package, error) {
	fs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	if len(fs) == 0 {
		return nil, fmt.Errorf("no package found in hashdir: %s", dir)
	}

	if len(fs) > 1 {
		return nil, fmt.Errorf("found multiple packages in hashdir: %s", dir)
	}

	return LoadPackageFile(path.Join(dir, fs[0].Name(), PkgFileName))
}
