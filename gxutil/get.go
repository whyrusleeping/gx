package gxutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

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
		}

		stump.VLog("Target directory already exists but isn't a valid package, cleaning up...")
		if oErr := os.RemoveAll(out); oErr != nil {
			stump.Error("cannot purge existing target directory:", oErr)
			return nil, oErr
		}
	}

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if err := pm.tryFetch(hash, out); err != nil {
		return nil, err
	}

	err = FindPackageInDir(&pkg, out)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

func (pm *PM) CacheAndLinkPackage(ref, cacheloc, out string) error {
	if err := pm.tryFetch(ref, cacheloc); err != nil {
		return err
	}

	finfo, err := os.Lstat(out)
	switch {
	case err == nil:
		if finfo.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(out)
			if err != nil {
				return err
			}

			// expecting 'ref' to be '/ipfs/QmFoo/pkg'
			if target != cacheloc {
				panic("not handling dep changes yet")
			}

			// Link already exists
			return nil
		} else {
			// TODO: should we force these to be links?
			// we want to support people just cloning packages into place here, so how should we handle it here?
			panic("not yet handling non-linked packages...")
		}
	case os.IsNotExist(err):
		// ok
	default:
		return err
	}

	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}

	// dir where the link goes
	linkloc, _ := filepath.Split(out)
	// relative path from link to cache
	rel, err := filepath.Rel(linkloc, cacheloc)
	if err != nil {
		return err
	}
	return os.Symlink(rel, out)
}

func (pm *PM) tryFetch(hash, target string) error {
	temp := target + ".part"

	// check if already downloaded
	_, err := os.Stat(target)
	if err == nil {
		stump.VLog("already fetched %s", target)
		return nil
	}

	// check if a fetch was previously started and failed, cleanup if found
	_, err = os.Stat(temp)
	if err == nil {
		stump.VLog("Found previously failed fetch, cleaning up...")
		if err := os.RemoveAll(temp); err != nil {
			stump.Error("cleaning up previous aborted transfer: %s", err)
		}
	}

	begin := time.Now()
	stump.VLog("  - fetching %s via ipfs api", hash)
	defer func() {
		stump.VLog("  - fetch finished in %s", time.Since(begin))
	}()
	tries := 3
	for i := 0; i < tries; i++ {
		if err := pm.Shell().Get(hash, temp); err != nil {
			stump.Error("from shell.Get(): %v", err)

			rmerr := os.RemoveAll(temp)
			if rmerr != nil {
				stump.Error("cleaning up temp download directory: %s", rmerr)
			}

			if i == tries-1 {
				return err
			}
			stump.Log("retrying fetch %s after a second...", hash)
			time.Sleep(time.Second)
		} else {
			/*
				if err := chmodR(temp, 0444); err != nil {
					return err
				}
			*/
			return os.Rename(temp, target)
		}
	}
	panic("unreachable")
}

func chmodR(dir string, perm os.FileMode) error {
	return filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if p == dir {
			return nil
		}

		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}

			perm := perm
			if info.IsDir() {
				perm |= 0111
			}

			return os.Chmod(p, perm)
		}
		return nil
	})
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
