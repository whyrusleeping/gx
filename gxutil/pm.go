package gxutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	sh "github.com/ipfs/go-ipfs-api"
	mh "github.com/jbenet/go-multihash"
	. "github.com/whyrusleeping/stump"
)

const PkgFileName = "package.json"

type PM struct {
	ipfssh *sh.Shell

	cfg *Config

	// hash of the 'empty' ipfs dir to avoid extra calls to object new
	blankDir string
}

func NewPM(cfg *Config) (*PM, error) {
	return &PM{
		ipfssh: NewShell(),
		cfg:    cfg,
	}, nil
}

func (pm *PM) Shell() *sh.Shell {
	if pm.ipfssh == nil {
		pm.ipfssh = NewShell()
	}

	return pm.ipfssh
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
			VLog("  - package %s already processed", dep.Name)
			continue
		}

		// if its already local, skip it
		pkgdir := filepath.Join(location, "gx", "ipfs", dep.Hash)
		cpkg := new(Package)
		err := FindPackageInDir(cpkg, pkgdir)
		if err != nil {
			VLog("  - %s not found locally, fetching: %s into %s", dep.Name, dep.Hash, pkgdir)
			deppkg, err := pm.GetPackageTo(dep.Hash, pkgdir)
			if err != nil {
				return fmt.Errorf("failed to fetch package: %s (%s):%s", dep.Name,
					dep.Hash, err)
			}
			VLog("  - fetch complete!")
			cpkg = deppkg
		}

		VLog("  - now processing dep %s-%s of %s", dep.Name, dep.Hash, pkg.Name)
		err = pm.installDepsRec(cpkg, location, done)
		if err != nil {
			return err
		}

		err = TryRunHook("post-install", cpkg.Language, pkgdir)
		if err != nil {
			return err
		}
		done[dep.Hash] = struct{}{}
	}
	return nil
}

func (pm *PM) InitPkg(dir, name, lang string, setup func(*Package)) error {
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
	pkg.Author = username
	pkg.Language = lang
	pkg.Version = "0.0.0"

	if setup != nil {
		setup(pkg)
	}

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
	pkgpath := filepath.Join(dir, dephash)
	// check if its already imported
	_, err := os.Stat(pkgpath)
	if err == nil {
		var pkg Package
		err := FindPackageInDir(&pkg, pkgpath)
		if err != nil {
			return nil, fmt.Errorf("dir for package already exists, but no package found:", err)
		}

		return &Dependency{
			Name:    pkg.Name,
			Hash:    dephash,
			Version: pkg.Version,
		}, nil
	}

	ndep, err := pm.GetPackageTo(dephash, pkgpath)
	if err != nil {
		return nil, err
	}

	err = TryRunHook("post-install", ndep.Language, pkgpath)
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
		return nil, err
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

func LocalPackageByName(dir, name string, out interface{}) error {
	if IsHash(name) {
		return FindPackageInDir(out, filepath.Join(dir, name))
	}

	var local Package
	err := LoadPackageFile(&local, PkgFileName)
	if err != nil {
		return err
	}

	return resolveDepName(&local, out, dir, name, make(map[string]struct{}))
}

var ErrUnrecognizedName = fmt.Errorf("unrecognized package name")

func resolveDepName(pkg *Package, out interface{}, dir, name string, checked map[string]struct{}) error {
	// first check if its a direct dependency of this package
	for _, d := range pkg.Dependencies {
		if d.Name == name {
			return LoadPackageFile(out, filepath.Join(dir, d.Hash, d.Name, PkgFileName))
		}
	}

	// recurse!
	var dpkg Package
	for _, d := range pkg.Dependencies {
		if _, ok := checked[d.Hash]; ok {
			continue
		}

		err := LoadPackageFile(&dpkg, filepath.Join(dir, d.Hash, d.Name, PkgFileName))
		if err != nil {
			return err
		}

		err = resolveDepName(&dpkg, out, dir, name, checked)
		switch err {
		case nil:
			return nil // success!
		case ErrUnrecognizedName:
			checked[d.Hash] = struct{}{}
		default:
			return err
		}
	}

	return ErrUnrecognizedName
}

func TryRunHook(hook, env string, args ...string) error {
	if env == "" {
		return nil
	}

	binname := "gx-" + env
	_, err := exec.LookPath(binname)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			VLog("runhook(%s): No gx helper tool found for", hook, env)
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
		return fmt.Errorf("%s hook failed: %s", hook, err)
	}

	return nil
}

func InstallPath(env, relpath string, global bool) (string, error) {
	if env == "" {
		Error("no env, returning empty install path")
		return "", nil
	}

	binname := "gx-" + env
	_, err := exec.LookPath(binname)
	if err != nil {
		if strings.Contains(err.Error(), "file not found") {
			VLog("runhook(install-path): No gx helper tool found for", env)
			return "", nil
		}
		return "", err
	}

	args := []string{"hook", "install-path"}
	if global {
		args = append(args, "--global")
	}
	cmd := exec.Command(binname, args...)

	cmd.Dir = relpath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("install-path hook failed: %s", err)
	}

	return strings.Trim(string(out), " \t\n"), nil
}

func IsHash(s string) bool {
	return strings.HasPrefix(s, "Qm") && len(s) == 46
}
