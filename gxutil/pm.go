package gxutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	mh "github.com/jbenet/go-multihash"
	ish "github.com/whyrusleeping/fallback-ipfs-shell"
	. "github.com/whyrusleeping/stump"
)

const PkgFileName = "package.json"

type PM struct {
	shell ish.Shell

	cfg *Config

	// hash of the 'empty' ipfs dir to avoid extra calls to object new
	blankDir string
}

func NewPM(cfg *Config) (*PM, error) {
	sh, err := ish.NewShell()
	if err != nil {
		return nil, err
	}

	return &PM{
		shell: sh,
		cfg:   cfg,
	}, nil
}

// InstallDeps recursively installs all dependencies for the given package
func (pm *PM) InstallDeps(pkg *Package, location string) error {
	done := make(map[string]struct{})
	return pm.installDepsRec(pkg, location, done)
}

func (pm *PM) installDepsRec(pkg *Package, location string, done map[string]struct{}) error {
	Log("installing package: %s-%s", pkg.Name, pkg.Version)
	for _, dep := range pkg.Dependencies {
		if _, ok := done[dep.Hash]; ok {
			continue
		}

		// if its already local, skip it
		pkgdir := filepath.Join(location, dep.Hash)
		pkg := new(Package)
		err := FindPackageInDir(pkg, pkgdir)
		if err != nil {
			deppkg, err := pm.GetPackageLocalDaemon(dep.Hash, location)
			if err != nil {
				return fmt.Errorf("failed to fetch package: %s (%s):%s", dep.Name,
					dep.Hash, err)
			}
			pkg = deppkg
		}

		err = pm.installDepsRec(pkg, location, done)
		if err != nil {
			return err
		}
		done[dep.Hash] = struct{}{}
	}
	return nil
}

func (pm *PM) InitPkg(dir, name, lang string) error {
	// check for existing packagefile
	p := filepath.Join(dir, PkgFileName)
	_, err := os.Stat(p)
	if err == nil {
		return errors.New("package file already exists in working dir")
	}

	username := pm.cfg.User.Name
	if username == "" {
		u, err := user.Current()
		if err != nil {
			fmt.Errorf("error looking up current user: %s", err)
		}
		username = u.Username
	}

	pkg := new(Package)
	pkg.Name = name
	pkg.Language = lang
	pkg.Author = username
	pkg.Version = "1.0.0"

	// check if the user has a tool installed for the selected language
	CheckForHelperTools(lang)

	err = SavePackageFile(pkg, p)
	if err != nil {
		return err
	}

	err = TryRunHook("post-init", lang, dir)
	if err != nil {
		return err
	}
	return nil
}

func CheckForHelperTools(lang string) {
	_, err := exec.LookPath("gx-" + lang)
	if err == nil {
		return
	}

	if strings.Contains(err.Error(), "file not found") {
		Log("notice: no helper tool found for", lang)
		return
	}

	Error("checking for helper tool:", err)
}

// ImportPackage downloads the package specified by dephash into the package
// in the directory 'dir'
func (pm *PM) ImportPackage(dir, dephash string) (*Dependency, error) {
	ndep, err := pm.GetPackage(dephash)
	if err != nil {
		return nil, err
	}

	for _, child := range ndep.Dependencies {
		_, err := pm.ImportPackage(dir, child.Hash)
		if err != nil {
			return nil, err
		}
	}

	err = TryRunHook("post-import", ndep.Language, dephash)
	if err != nil {
		Fatal(err)
	}

	return &Dependency{
		Name:    ndep.Name,
		Hash:    dephash,
		Version: ndep.Version,
	}, nil
}

// ResolveDepName resolves a given package name to a hash
// using configured repos as a mapping.
func (pm *PM) ResolveDepName(name string) (string, error) {
	_, err := mh.FromB58String(name)
	if err == nil {
		return name, nil
	}

	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		rpath, ok := pm.cfg.GetRepos()[parts[0]]
		if !ok {
			return "", fmt.Errorf("unknown repo: '%s'", parts[0])
		}

		pkgs, err := pm.FetchRepo(rpath, true)
		if err != nil {
			return "", err
		}

		val, ok := pkgs[parts[1]]
		if !ok {
			return "", fmt.Errorf("package %s not found in repo %s", parts[1], parts[0])
		}

		return val, nil
	}

	out, err := pm.QueryRepos(name)
	if err != nil {
		return "", err
	}

	if len(out) == 0 {
		return "", fmt.Errorf("could not find package by name: %s", name)
	}

	if len(out) == 1 {
		for _, v := range out {
			return v, nil
		}
	}

	return "", fmt.Errorf("ambiguous ref, appears in multiple repos")
}

func (pm *PM) EnumerateDependencies(pkg *Package) (map[string]struct{}, error) {
	deps := make(map[string]struct{})
	err := pm.enumerateDepsRec(pkg, deps)
	if err != nil {
		return nil, err
	}

	return deps, nil
}

func (pm *PM) enumerateDepsRec(pkg *Package, set map[string]struct{}) error {
	for _, d := range pkg.Dependencies {
		set[d.Hash] = struct{}{}

		depkg, err := pm.GetPackage(d.Hash)
		if err != nil {
			return err
		}

		err = pm.enumerateDepsRec(depkg, set)
		if err != nil {
			return err
		}
	}
	return nil
}

func TryRunHook(hook, env string, args ...string) error {
	if env == "" {
		return nil
	}

	binname := "gx-" + env
	_, err := exec.LookPath(binname)
	if err != nil {
		if os.IsNotExist(err) {
			Log("No gx helper tool found for", env)
			return nil
		}
		return err
	}

	args = append([]string{"hook", hook}, args...)
	cmd := exec.Command(binname, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("hook failed: %s", err)
	}

	return nil
}
