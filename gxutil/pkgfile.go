package gxutil

import (
	"io/ioutil"
	"encoding/json"
	"fmt"
	"os"

	log "github.com/whyrusleeping/stump"
)

type PackageBase struct {
	Name         string        `json:"name,omitempty"`
	Author       string        `json:"author,omitempty"`
	Description  string        `json:"description,omitempty"`
	Keywords     []string      `json:"keywords,omitempty"`
	Version      string        `json:"version,omitempty"`
	Dependencies []*Dependency `json:"gxDependencies,omitempty"`
	Bin          string        `json:"bin,omitempty"`
	Build        string        `json:"build,omitempty"`
	Test         string        `json:"test,omitempty"`
	Language     string        `json:"language,omitempty"`
	License      string        `json:"license"`
	Issues       *Issues       `json:"bugs"`
	GxVersion    string        `json:"gxVersion"`
	Repository   *Repository   `json:"repository,omitempty"`
	NonGxFields map[string]interface{} `json:"-"`
}

type Package struct {
	PackageBase

	Gx json.RawMessage `json:"gx,omitempty"`
}

type Repository struct {
	Type  string `json:"type"`
	Url   string `json:"url"`
}

type Issues struct {
	Url string `json:"url"`
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

	file, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil
	}
	json.Unmarshal(file, &pkg.(*Package).NonGxFields)


	return nil
}

func SavePackageFile(pkg interface{}, fname string, nonGxFields map[string]interface{}) error {
	fi, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer fi.Close()

	base, err := json.Marshal(pkg)
	if (err != nil) {
		return err
	}
	var f map[string]interface{}
	json.Unmarshal(base, &f)
	var g map[string]interface{} = nonGxFields
	if g == nil {
		g = make(map[string]interface{})
	}

	for v, k := range f {
		g[v] = k
	}
	out, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	_, err = fi.Write(out)
	return err
}

// FindDep returns a reference to the named dependency in this package file
func (pkg *PackageBase) FindDep(ref string) *Dependency {
	for _, d := range pkg.Dependencies {
		if d.Hash == ref || d.Name == ref {
			return d
		}
	}
	return nil
}

func (pkg *PackageBase) ForEachDep(cb func(dep *Dependency, pkg *Package) error) error {
	log.VLog("  - foreachdep: %s", pkg.Name)
	for _, dep := range pkg.Dependencies {
		var cpkg Package
		err := LoadPackage(&cpkg, pkg.Language, dep.Hash)
		if err != nil {
			log.VLog("LoadPackage error: ", err)
			return fmt.Errorf("package %s (%s) not found", dep.Name, dep.Hash)
		}

		err = cb(dep, &cpkg)
		if err != nil {
			return err
		}
	}

	return nil
}
