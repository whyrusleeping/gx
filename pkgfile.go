package main

import (
	"encoding/json"
	"os"
)

type Package struct {
	Name         string
	Files        []string
	Version      string
	Dependencies []*Dependency
	Bin          string
	Build        string
	Test         string
	Language     string
}

// Dependency represents a dependency of a package
type Dependency struct {
	Author string
	Name   string
	Hash   string
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

	out, err := json.MarshalIndent(pkg, "", "\t")
	if err != nil {
		return err
	}
	_, err = fi.Write(out)
	return err
}
