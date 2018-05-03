package main

import (
	"fmt"

	gx "github.com/whyrusleeping/gx/gxutil"
)

type pkgImport struct {
	parents []string
	version string
}

func check(pkg *gx.Package) (bool, error) {
	var failed bool
	// name -> hash -> path+version
	packages := make(map[string]map[string]*pkgImport)

	var traverse func(*gx.Package) error
	traverse = func(pkg *gx.Package) error {
		return pkg.ForEachDep(func(dep *gx.Dependency, dpkg *gx.Package) error {
			pkgVersions, ok := packages[dpkg.Name]
			if !ok {
				pkgVersions = make(map[string]*pkgImport, 1)
				packages[dpkg.Name] = pkgVersions
			}
			imp, ok := pkgVersions[dep.Hash]
			if !ok {
				pkgVersions[dep.Hash] = &pkgImport{
					version: dpkg.Version,
					parents: []string{pkg.Name},
				}
				return traverse(dpkg)
			}
			imp.parents = append(imp.parents, pkg.Name)
			return nil
		})
	}
	if err := traverse(pkg); err != nil {
		return !failed, err
	}
	for name, pkgVersions := range packages {
		switch len(pkgVersions) {
		case 0:
			panic("must have at least one package version")
		case 1:
			continue
		}
		failed = true

		fmt.Printf("package %s imported as:\n", name)
		for hash, imp := range pkgVersions {
			fmt.Printf("  - %s %s\n", imp.version, hash)
			for _, p := range imp.parents {
				fmt.Printf("    - %s\n", p)
			}
		}
	}
	// Finally, check names and versions.
	if err := pkg.ForEachDep(func(dep *gx.Dependency, dpkg *gx.Package) error {
		if dep.Name != dpkg.Name {
			failed = true
			fmt.Printf(
				"dependency %s references a package with name %s\n",
				dep.Name,
				dpkg.Name,
			)
		}
		if dep.Version != dpkg.Version {
			failed = true
			fmt.Printf(
				"dependency %s has version %s but the referenced package has version %s\n",
				dep.Name,
				dep.Version,
				dpkg.Version,
			)
		}
		return nil
	}); err != nil {
		return !failed, err
	}
	return !failed, nil
}
