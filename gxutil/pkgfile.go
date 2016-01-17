package gxutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PackageBase struct {
	Name         string        `json:"name,omitempty"`
	Author       string        `json:"author,omitempty"`
	Description  string        `json:"description,omitempty"`
	Keywords     string        `json:"keywords,omitempty"`
	Version      string        `json:"version,omitempty"`
	Dependencies []*Dependency `json:"gxDependencies,omitempty"`
	Bin          string        `json:"bin,omitempty"`
	Build        string        `json:"build,omitempty"`
	Test         string        `json:"test,omitempty"`
	Language     string        `json:"language,omitempty"`
	Copyright    string        `json:"copyright,omitempty"`
	Issues       string        `json:"issues_url"`
}

type Package struct {
	PackageBase

	Gx json.RawMessage `json:"gx,omitempty"`
}

// Dependency represents a dependency of a package
type Dependency struct {
	Author  string `json:"author,omitempty"`
	Name    string `json:"name,omitempty"`
	Hash    string `json:"hash"`
	Version string `json:"version,omitempty"`
}

func LoadPackageFile(pkg interface{}, fname string) error {
	fi, err := os.Open(fname)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(fi)
	err = dec.Decode(pkg)
	if err != nil {
		return err
	}

	return nil
}

func SavePackageFile(pkg interface{}, fname string) error {
	fi, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer fi.Close()

	out, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	_, err = fi.Write(out)
	return err
}

func (pkg *PackageBase) FindDep(ref string) *Dependency {
	for _, d := range pkg.Dependencies {
		if d.Hash == ref || d.Name == ref {
			return d
		}
	}
	return nil
}

func (pkg *PackageBase) ForEachDep(cb func(dep *Dependency, pkg *Package) error) error {
	local, err := InstallPath(pkg.Language, "", false)
	if err != nil {
		return err
	}

	global, err := InstallPath(pkg.Language, "", true)
	if err != nil {
		return err
	}

	for _, dep := range pkg.Dependencies {
		var pkg Package
		err := FindPackageInDir(&pkg, filepath.Join(local, "gx", "ipfs", dep.Hash))
		if err == nil {
			err := cb(dep, &pkg)
			if err != nil {
				return err
			}

			continue
		}
		err = FindPackageInDir(&pkg, filepath.Join(global, "gx", "ipfs", dep.Hash))
		if err != nil {
			return fmt.Errorf("package %s (%s) not found", dep.Name, dep.Hash)
		}

		err = cb(dep, &pkg)
		if err != nil {
			return err
		}
	}

	return nil
}
