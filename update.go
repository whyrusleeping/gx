package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	gx "github.com/whyrusleeping/gx/gxutil"
	log "github.com/whyrusleeping/stump"
)

func RecursiveDepUpdate(pkg *gx.Package, from, to string) error {
	log.Log("recursively updating %s to %s", from, to)
	todo := map[string]string{
		from: to,
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	checked := make(map[string]bool)

	_, err = cascadingUpdate(pkg, cwd, todo, checked)
	if err != nil {
		return err
	}

	return nil
}

func cascadingUpdate(cur *gx.Package, dir string, updates map[string]string, checked map[string]bool) (bool, error) {
	log.Log("cascading update of package %s in %s", cur.Name, dir)
	var changed bool
	err := cur.ForEachDep(func(dep *gx.Dependency, child *gx.Package) error {
		if checked[dep.Hash] {
			return nil
		}
		log.Log("  - processing %s...", dep.Name)

		if to, ok := updates[dep.Hash]; ok {
			log.Log(" ==> updating dep %s on %s", dep.Name, cur.Name)
			dep.Hash = to
			dep.Version = child.Version
			changed = true
		} else {
			nchild, err := fetchAndUpdate(dep.Hash, updates, checked)
			if err != nil {
				return err
			}

			if nchild != "" {
				updates[dep.Hash] = nchild
				dep.Hash = nchild
				changed = true
			} else {
				checked[dep.Hash] = true
			}
		}

		return nil
	})

	if err != nil {
		return false, err
	}

	return changed, nil
}

func fetchAndUpdate(tofetch string, updates map[string]string, checked map[string]bool) (string, error) {
	log.Log("fetch and update: %s", tofetch)
	dir, err := ioutil.TempDir("", "gx-update")
	if err != nil {
		return "", err
	}

	dir = filepath.Join(dir, tofetch)

	pkg, err := pm.GetPackageTo(tofetch, dir)
	if err != nil {
		return "", err
	}

	changed, err := cascadingUpdate(pkg, dir, updates, checked)
	if err != nil {
		return "", err
	}

	if changed {
		err := gx.SavePackageFile(pkg, filepath.Join(dir, pkg.Name, gx.PkgFileName))
		if err != nil {
			return "", err
		}

		return pm.PublishPackage(dir, &pkg.PackageBase)
	}

	return "", nil
}
