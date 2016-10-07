package gxutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gi "github.com/sabhiram/go-git-ignore"
)

func (pm *PM) PublishPackage(dir string, pkg *PackageBase) (string, error) {
	// make sure we have the actual package dir, and not a hashdir
	if _, err := os.Stat(filepath.Join(dir, PkgFileName)); err != nil {
		// try appending the package name
		_, err = os.Stat(filepath.Join(dir, pkg.Name, PkgFileName))
		if err != nil {
			return "", fmt.Errorf("%s did not contain a package!")
		}
		dir = filepath.Join(dir, pkg.Name)
	}

	gitig, err := gi.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	gxig, err := gi.CompileIgnoreFile(filepath.Join(dir, ".gxignore"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	var files []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {

		// ignore directories
		if info.IsDir() {
			return nil
		}

		// get relative path
		rel := p[len(dir):]
		if dir[len(dir)-1] != '/' {
			rel = rel[1:]
		}

		// make relative path cross platform safe
		rel = filepath.ToSlash(rel)

		// respect gitignore
		if gitig != nil && gitig.MatchesPath(rel) {
			return nil
		}

		// respect gxignore
		if gxig != nil && gxig.MatchesPath(rel) {
			return nil
		}

		// dont publish the git repo
		if strings.HasPrefix(rel, ".git") {
			return nil
		}

		// dont publish vendored code
		if strings.HasPrefix(rel, "vendor") {
			return nil
		}

		// dont publish gx repo files
		if strings.HasPrefix(rel, ".gx/") || strings.HasSuffix(rel, ".gxrc") {
			return nil
		}

		files = append(files, rel)
		return nil
	})

	// we cant guarantee that the 'empty dir' object exists already
	blank, err := pm.Shell().NewObject("unixfs-dir")
	if err != nil {
		return "", err
	}

	pm.blankDir = blank

	pkgdir, err := pm.addFiles(dir, files)
	if err != nil {
		return "", err
	}

	final, err := pm.Shell().Patch(pm.blankDir, "add-link", pkg.Name, pkgdir)
	if err != nil {
		return "", err
	}

	return final, pm.Shell().Pin(final)
}

type filetree struct {
	children map[string]*filetree
}

func newFiletree() *filetree {
	return &filetree{make(map[string]*filetree)}
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
			child = newFiletree()
			ft.children[path[0]] = child
		}

		return child.insert(path[1:])
	}

	if len(path) == 1 {
		_, ok := ft.children[path[0]]
		if ok {
			return fmt.Errorf("path already exists: %s", path[0])
		}

		ft.children[path[0]] = newFiletree()
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

func (pm *PM) addFile(p string) (string, error) {
	fi, err := os.Open(p)
	if err != nil {
		fmt.Printf("open failed: %s\n", err)
		return "", err
	}
	defer fi.Close()

	return pm.Shell().Add(fi)
}

func (pm *PM) addPathElem(v *filetree, f, cwd string) (string, error) {
	if v == nil || len(v.children) == 0 {

		// file or symlink here
		p := filepath.Join(cwd, f)
		stat, err := os.Lstat(p)
		if err != nil {
			fmt.Printf("file stat failed: %s\n", err)
			return "", err
		}

		if stat.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return "", err
			}

			return pm.Shell().AddLink(target)
		}

		return pm.addFile(p)
	}

	return pm.addTree(v, filepath.Join(cwd, f))
}

func (pm *PM) addTree(nd *filetree, cwd string) (string, error) {
	cur := pm.blankDir
	for f, v := range nd.children {
		hash, err := pm.addPathElem(v, f, cwd)
		if err != nil {
			return "", err
		}
		patched, err := pm.Shell().Patch(cur, "add-link", f, hash)
		if err != nil {
			return "", err
		}

		cur = patched
	}

	return cur, nil
}
