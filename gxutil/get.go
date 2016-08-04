package gxutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	rname "github.com/jbenet/go-os-rename"
	stump "github.com/whyrusleeping/stump"
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

	outtemp := out + ".part"

	// check if a fetch was previously started and failed, cleanup if found
	_, err = os.Stat(outtemp)
	if err == nil {
		stump.VLog("Found previously failed fetch, cleaning up...")
		if err := os.RemoveAll(outtemp); err != nil {
			stump.Error("cleaning up previous aborted transfer: %s", err)
		}
	}

	begin := time.Now()
	stump.VLog("  - fetching %s via ipfs api", hash)
	tries := 3
	for i := 0; i < tries; i++ {
		if err := pm.Shell().Get(hash, outtemp); err != nil {
			stump.Error("from shell.Get(): %v", err)

			rmerr := os.RemoveAll(outtemp)
			if rmerr != nil {
				stump.Error("cleaning up temp download directory: %s", rmerr)
			}

			if i == tries-1 {
				return nil, err
			}
			stump.Log("retrying fetch %s after a second...", hash)
			time.Sleep(time.Second)
		} else {
			if err := rname.Rename(outtemp, out); err != nil {
				return nil, err
			}
			break
		}
	}
	stump.VLog("  - fetch finished in %s", time.Now().Sub(begin))

	err = FindPackageInDir(&pkg, out)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

func FindPackageInDir(pkg interface{}, dir string) error {
	if err := LoadPackageFile(pkg, filepath.Join(dir, PkgFileName)); err == nil {
		return nil
	}

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
