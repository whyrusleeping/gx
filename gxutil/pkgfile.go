package gxutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/whyrusleeping/stump"
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
	License      string        `json:"license"`
	Issues       string        `json:"bugs"`
	GxVersion    string        `json:"gxVersion"`
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
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, pkg)
}

func SavePackageFile(pkg interface{}, fname string) error {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return writeJson(pkg, fname)
		}
		return err
	}

	var current map[string]interface{}
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(pkg); err != nil {
		return err
	}

	var modified map[string]interface{}
	if err := json.NewDecoder(buf).Decode(&modified); err != nil {
		return err
	}

	return writeJson(mergeMaps(current, modified), fname)
}

func writeJson(i interface{}, fname string) error {
	fi, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer fi.Close()

	out, err := json.MarshalIndent(i, "", "  ")
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
