package gxutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	. "github.com/whyrusleeping/stump"
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

func (pm *PM) GetPackageTo(hash, out string) (*Package, error) {
	var pkg Package
	_, err := os.Stat(out)
	if err == nil {
		err := FindPackageInDir(&pkg, out)
		if err == nil {
			return &pkg, nil
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	begin := time.Now()
	VLog("  - fetching %s via ipfs api", hash)
	err = pm.Shell().Get(hash, out)
	if err != nil {
		return nil, err
	}
	VLog("  - fetch finished in %s", time.Now().Sub(begin))

	err = FindPackageInDir(&pkg, out)
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
