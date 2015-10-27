package gxutil

import (
	"fmt"
	"os"
	"path"
	"strings"
)

func packagesGoImport(p string) (string, error) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		return "", fmt.Errorf("GOPATH not set, cannot derive import path")
	}

	srcdir := path.Join(gopath, "src")
	srcdir += "/"

	if !strings.HasPrefix(p, srcdir) {
		return "", fmt.Errorf("package not within GOPATH/src")
	}

	return p[len(srcdir):], nil
}
