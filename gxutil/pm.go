package gxutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"

	sh "github.com/ipfs/go-ipfs-api"
)

const PkgFileName = "package.json"

var ErrLinkAlreadyExists = fmt.Errorf("named package link already exists")

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
		_, err := findPackageInDir(pkgdir)
		if err == nil {
			continue
		}

		deppkg, err := pm.GetPackageLocalDaemon(dep.Hash, location)
		if err != nil {
			return fmt.Errorf("failed to fetch package: %s (%s):%s", dep.Name,
				dep.Hash, err)
		}

		err = pm.CheckRequirements(deppkg)
		if err != nil {
			return err
		}

		err = pm.InstallDeps(pkg, location)
		if err != nil {
			return err
		}
	}
	return nil
}

func (pm *PM) CheckRequirements(pkg *Package) error {
	switch pkg.Language {
	case "go":
		if pkg.Go != nil && pkg.Go.GoVersion != "" {
			out, err := exec.Command("go", "version").CombinedOutput()
			if err != nil {
				return fmt.Errorf("no go compiler installed")
			}

			parts := strings.Split(string(out), " ")
			if len(parts) < 4 {
				return fmt.Errorf("unrecognized output from go compiler")
			}

			havevers := parts[2][2:]

			reqvers := pkg.Go.GoVersion

			badreq, err := versionComp(havevers, reqvers)
			if err != nil {
				return err
			}
			if badreq {
				return fmt.Errorf("package '%s' requires go version %s, you have %s installed.", pkg.Name, reqvers, havevers)
			}

		}
		return nil

	default:
		return nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func versionComp(have, req string) (bool, error) {
	hp := strings.Split(have, ".")
	rp := strings.Split(req, ".")

	l := min(len(hp), len(rp))
	hp = hp[:l]
	rp = rp[:l]
	for i, v := range hp {
		hv, err := strconv.Atoi(v)
		if err != nil {
			return false, err
		}

		rv, err := strconv.Atoi(rp[i])
		if err != nil {
			return false, err
		}

		if hv < rv {
			return true, nil
		}
	}
	return false, nil
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

func RemoveLink(dir, hash, name string) error {
	linkpath := path.Join(dir, name)
	finfo, err := os.Lstat(linkpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	if finfo.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s was not a package link", name)
	}

	return os.Remove(linkpath)
}

func TryLinkPackage(dir, hash, name string) error {
	finfo, err := os.Lstat(path.Join(dir, name))
	if err == nil {
		if finfo.Mode()&os.ModeSymlink != 0 {
			return ErrLinkAlreadyExists
		} else {
			return fmt.Errorf("link target exists and is not a symlink")
		}
	}

	if !os.IsNotExist(err) {
		return err
	}

	pkgname, err := packageNameInDir(path.Join(dir, hash))
	if err != nil {
		return err
	}

	return os.Symlink(path.Join(hash, pkgname), path.Join(dir, name))
}

func (pm *PM) InitPkg(dir, name, lang string) error {
	// check for existing packagefile
	p := path.Join(dir, PkgFileName)
	_, err := os.Stat(p)
	if err == nil {
		return errors.New("package file already exists in working dir")
	}

	username := pm.cfg.getUsername()
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
