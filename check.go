package main

import (
	"fmt"
	"sort"

	gx "github.com/whyrusleeping/gx/gxutil"

	"github.com/blang/semver"
)

type pkgImport struct {
	parents []string
	version semver.Version
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
				version, err := semver.Parse(dpkg.Version)
				if dpkg.Version != "" && err != nil {
					fmt.Printf(
						"package %s (%s) has an invalid version '%s': %s\n",
						dpkg.Name,
						dep.Hash,
						dpkg.Version,
						err,
					)
				}
				pkgVersions[dep.Hash] = &pkgImport{
					version: version,
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

	var dupes []string
	for name, pkgVersions := range packages {
		switch len(pkgVersions) {
		case 0:
			panic("must have at least one package version")
		case 1:
			continue
		}
		dupes = append(dupes, name)
	}

	if len(dupes) > 0 {
		failed = true
		sort.Strings(dupes)

		for _, name := range dupes {
			pkgVersions := packages[name]

			hashes := make([]string, 0, len(pkgVersions))
			for h := range pkgVersions {
				hashes = append(hashes, h)
			}

			sort.Slice(hashes, func(i, j int) bool {
				ih := hashes[i]
				jh := hashes[j]
				iv := pkgVersions[ih].version
				jv := pkgVersions[jh].version
				if res := iv.Compare(jv); res != 0 {
					return res < 0
				}
				return ih < jh
			})

			fmt.Printf("package %s imported as:\n", name)
			for _, hash := range hashes {
				imp := pkgVersions[hash]

				fmt.Printf("  - %s %s\n", imp.version, hash)
				sort.Strings(imp.parents)
				for _, p := range imp.parents {
					fmt.Printf("    - %s\n", p)
				}
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
