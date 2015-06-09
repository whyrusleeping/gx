package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
)

func (pm *PM) PublishPackage(dir string, pkg *Package) (string, error) {
	files := pkg.Files
	found := false
	for _, f := range files {
		if f == PkgFileName {
			found = true
			break
		}
	}

	if !found {
		files = append(files, PkgFileName)
	}

	// we cant guarantee that the 'empty dir' object exists already
	blank, err := pm.shell.NewObject("unixfs-dir")
	if err != nil {
		return "", err
	}

	pm.blankDir = blank

	pkgdir, err := pm.addFiles(dir, files)
	if err != nil {
		return "", err
	}

	return pm.shell.Patch(pm.blankDir, "add-link", pkg.Name, pkgdir)
}

type filetree struct {
	children map[string]*filetree
}

func newFiletreeFromFiles(files []string) (*filetree, error) {
	root := &filetree{make(map[string]*filetree)}
	for _, f := range files {
		f = strings.TrimRight(f, "/")
		parts := strings.Split(f, "/")
		if err := root.insert(parts); err != nil {
			return nil, err
		}
	}
	return root, nil
}

func (ft *filetree) insert(path []string) error {
	if len(path) > 1 {
		child, ok := ft.children[path[0]]
		if !ok {
			child = &filetree{make(map[string]*filetree)}
			ft.children[path[0]] = child
		}

		return child.insert(path[1:])
	}

	if len(path) == 1 {
		_, ok := ft.children[path[0]]
		if ok {
			return fmt.Errorf("path already exists: %s", path[0])
		}

		ft.children[path[0]] = nil
		return nil
	}

	panic("branch never reached")
}

func (pm *PM) addFiles(root string, files []string) (string, error) {
	tree, err := newFiletreeFromFiles(files)
	if err != nil {
		return "", err
	}

	return pm.addTree(tree, root)
}

func (pm *PM) addTree(nd *filetree, cwd string) (string, error) {
	cur := pm.blankDir
	for f, v := range nd.children {
		var hash string
		if v == nil {
			// file here
			fi, err := os.Open(path.Join(cwd, f))
			if err != nil {
				log.Printf("open failed: %s", err)
				return "", err
			}

			ch, err := pm.shell.Add(fi)
			if err != nil {
				fi.Close()
				return "", err
			}
			hash = ch
			fi.Close()
		} else {
			ch, err := pm.addTree(v, path.Join(cwd, f))
			if err != nil {
				return "", err
			}
			hash = ch
		}
		patched, err := pm.shell.Patch(cur, "add-link", f, hash)
		if err != nil {
			return "", err
		}

		cur = patched
	}

	return cur, nil
}
