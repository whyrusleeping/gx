package gxutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strings"

	sh "github.com/ipfs/go-ipfs-api"
	mh "github.com/jbenet/go-multihash"
	. "github.com/whyrusleeping/stump"
)

const PkgFileName = "package.json"

type PM struct {
	shell *sh.Shell

	cfg *Config

	// hash of the 'empty' ipfs dir to avoid extra calls to object new
	blankDir string
}

func NewPM(cfg *Config) *PM {
	return &PM{
		shell: sh.NewShell(getDaemonAddr()),
		cfg:   cfg,
	}
}

// InstallDeps recursively installs all dependencies for the given package
func (pm *PM) InstallDeps(pkg *Package, location string) error {
	fmt.Printf("installing package: %s-%s\n", pkg.Name, pkg.Version)
	for _, dep := range pkg.Dependencies {

		// if its already local, skip it
		pkgdir := path.Join(location, dep.Hash)
		_, err := FindPackageInDir(pkgdir)
		if err == nil {
			continue
		}

		deppkg, err := pm.GetPackageLocalDaemon(dep.Hash, location)
		if err != nil {
			return fmt.Errorf("failed to fetch package: %s (%s):%s", dep.Name,
				dep.Hash, err)
		}

		err = RunReqCheckHook(deppkg.Language, dep.Hash)
		if err != nil {
			return err
		}

		err = pm.InstallDeps(deppkg, location)
		if err != nil {
			return err
		}
	}
	return nil
}

func (pm *PM) CheckDaemon() error {
	_, err := pm.shell.ID()
	return err
}

func getDaemonAddr() string {
	da := os.Getenv("GX_IPFS_ADDR")
	if len(da) == 0 {
		return "localhost:5001"
	}
	return da
}

func (pm *PM) InitPkg(dir, name, lang string) error {
	// check for existing packagefile
	p := path.Join(dir, PkgFileName)
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

	switch lang {
	case "go":
		_, err := exec.LookPath("gx-go-tool")
		if err != nil {
			fmt.Println("gx-go-tool not found in path.")
			fmt.Println("this tool is recommended when using gx for go packages.")
			fmt.Println("to install, run: 'go get -u github.com/whyrusleeping/gx-go-tool'")
			fmt.Println()
		}

		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not determine cwd")
		}
		imp, _ := packagesGoImport(dir)

		if imp != "" {
			pkg.Go = &GoInfo{
				DvcsImport: imp,
			}
		}

	default:
	}

	return SavePackageFile(pkg, p)
}

func (pm *PM) ImportPackage(dir, dephash, name string, nolink bool) (*Dependency, error) {
	ndep, err := pm.GetPackage(dephash)
	if err != nil {
		return nil, err
	}

	if len(name) == 0 {
		name = ndep.Name + "-v" + ndep.Version
	}

	err = RunReqCheckHook(ndep.Language, dephash)
	if err != nil {
		return nil, err
	}

	return &Dependency{
		Name:    ndep.Name,
		Hash:    dephash,
		Version: ndep.Version,
	}, nil
}

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

func RunPostImportHook(env, pkg string) error {
	return TryRunHook("post-import", env, pkg)
}

func RunReqCheckHook(env, pkg string) error {
	return TryRunHook("req-check", env, pkg)
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
