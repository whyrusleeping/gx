package gxutil

import (
	"encoding/json"
	"os"
)

type Package struct {
	Name         string        `json:"name,omitempty"`
	Author       string        `json:"author,omitempty"`
	Version      string        `json:"version,omitempty"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
	Bin          string        `json:"bin,omitempty"`
	Build        string        `json:"build,omitempty"`
	Test         string        `json:"test,omitempty"`
	Language     string        `json:"language,omitempty"`
	Copyright    string        `json:"copyright,omitempty"`
}

// Dependency represents a dependency of a package
type Dependency struct {
	Author   string `json:"author,omitempty"`
	Name     string `json:"name,omitempty"`
	Hash     string `json:"hash"`
	Version  string `json:"version,omitempty"`
	Linkname string `json:"linkname,omitempty"`
}

func LoadPackageFile(fname string) (*Package, error) {
	fi, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(fi)
	var pkg Package
	err = dec.Decode(&pkg)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

func SavePackageFile(pkg *Package, fname string) error {
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

func (pkg *Package) FindDep(ref string) *Dependency {
	for _, d := range pkg.Dependencies {
		if d.Hash == ref || d.Name == ref {
			return d
		}
	}
	return nil
}
