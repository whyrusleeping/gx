package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gx "github.com/whyrusleeping/gx/gxutil"
)

func DiffPackages(a, b string) (*Diff, error) {
	dir, err := ioutil.TempDir("", "gx-diff")
	if err != nil {
		return nil, err
	}

	pa, err := pm.GetPackageTo(a, filepath.Join(dir, "a"))
	if err != nil {
		return nil, err
	}

	pb, err := pm.GetPackageTo(b, filepath.Join(dir, "b"))
	if err != nil {
		return nil, err
	}

	d, err := PkgFileDiff(dir, pa, pb)
	if err != nil {
		return nil, err
	}

	d.Hashes = []string{a, b}
	return d, nil
}

type Diff struct {
	Version []string
	Hashes  []string
	Name    string

	Imports map[string]*Diff

	dir string
}

func PkgFileDiff(dir string, a, b *gx.Package) (*Diff, error) {
	out := Diff{
		Version: []string{a.Version, b.Version},
		Name:    a.Name,
		Imports: make(map[string]*Diff),
		dir:     dir,
	}

	current := make(map[string]*gx.Dependency)

	for _, dep := range a.Dependencies {
		current[dep.Name] = dep
	}

	for _, dep := range b.Dependencies {
		old, ok := current[dep.Name]
		if ok {
			if old.Hash != dep.Hash {
				ddiff, err := DiffPackages(old.Hash, dep.Hash)
				if err != nil {
					return nil, err
				}

				out.Imports[dep.Name] = ddiff
			}
		} else {
			out.Imports[dep.Name] = &Diff{
				Version: []string{dep.Version},
				Hashes:  []string{dep.Hash},
			}
		}
	}

	return &out, nil
}

func (d *Diff) Print(interactive bool) {
	d.recPrint(interactive, make(map[string]bool))
}

func (d *Diff) recPrint(interactive bool, done map[string]bool) {
	if len(d.Version) == 2 {
		fmt.Printf("PACKAGE %s was changed from version\n", d.Name)
		fmt.Printf("  %s (%s)\n    to\n  %s (%s)\n", d.Version[0], d.Hashes[0], d.Version[1], d.Hashes[1])
	} else if len(d.Version) == 1 {
		fmt.Printf("PACKAGE %s was imported at version %s (%s)\n", d.Name, d.Version[0], d.Hashes[0])
		return
	}
	fmt.Printf("  There were %d changes in this packages dependencies.\n", len(d.Imports))
	if d.hasCodeChanges() {
		if !interactive || yesNoPrompt("  view code changes for this package?", true) {
			d.PrintCodeChanges()
		}
		fmt.Println()
	} else {
		fmt.Println("Nothing else was changed in this package.\n")
	}

	for _, cdiff := range d.Imports {
		n := strings.Join(cdiff.Hashes, "-")
		if !done[n] {
			cdiff.recPrint(interactive, done)
			done[n] = true
		}
	}
}

func (d *Diff) hasCodeChanges() bool {
	cmd := exec.Command("diff", "-r", "-x", "package.json", "a", "b")
	cmd.Dir = d.dir
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = ioutil.Discard
	err := cmd.Run()
	if err == nil {
		return false
	}

	return true
}

func (d *Diff) PrintCodeChanges() error {
	var cmd *exec.Cmd
	if _, err := exec.LookPath("git"); err == nil {
		err := ioutil.WriteFile(filepath.Join(d.dir, ".gitignore"), []byte("package.json"), 0664)
		if err != nil {
			return err
		}
		cmd = exec.Command("git", "diff", "a", "b")
	} else {
		cmd = exec.Command("diff", "-r", "-x", "package.json", "a", "b")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = d.dir
	return cmd.Run()
}

func (d *Diff) Cleanup() error {
	if d.dir != "" {
		err := os.RemoveAll(d.dir)
		if err != nil {
			return err
		}
	}

	for _, cdiff := range d.Imports {
		err := cdiff.Cleanup()
		if err != nil {
			return err
		}
	}

	return nil
}
