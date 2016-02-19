package gxutil

import (
	"os"

	sh "github.com/ipfs/go-ipfs-api"
	manet "github.com/jbenet/go-multiaddr-net"
	ma "github.com/jbenet/go-multiaddr-net/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
)

func NewShell() *sh.Shell {
	if apivar := os.Getenv("IPFS_API"); apivar != "" {
		return sh.NewShell(apivar)
	}

	ash, err := getLocalApiShell()
	if err == nil {
		return ash
	}

	return sh.NewShell("https://ipfs.io")
}

func getLocalApiShell() (*sh.Shell, error) {
	path, err := fsrepo.BestKnownPath()
	if err != nil {
		return nil, err
	}

	addr, err := fsrepo.APIAddr(path)
	if err != nil {
		return nil, err
	}

	host, err := multiaddrToNormal(addr)
	if err != nil {
		return nil, err
	}

	return sh.NewShell(host), nil
}

func multiaddrToNormal(addr string) (string, error) {
	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return "", err
	}

	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return "", err
	}

	return host, nil
}
