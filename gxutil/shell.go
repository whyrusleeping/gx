package gxutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	sh "github.com/ipfs/go-ipfs-api"
	homedir "github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	log "github.com/whyrusleeping/stump"
)

var UsingGateway bool

func NewShell() *sh.Shell {
	if apivar := os.Getenv("IPFS_API"); apivar != "" {
		log.VLog("using '%s' from IPFS_API env as api endpoint.", apivar)
		return sh.NewShell(apivar)
	}

	ash, err := getLocalAPIShell()
	if err == nil {
		return ash
	}

	UsingGateway = true

	log.VLog("using global ipfs gateways as api endpoint")
	return sh.NewShell("https://ipfs.io")
}

func getLocalAPIShell() (*sh.Shell, error) {
	ipath := os.Getenv("IPFS_PATH")
	if ipath == "" {
		home, err := homedir.Dir()
		if err != nil {
			return nil, err
		}

		ipath = filepath.Join(home, ".ipfs")
	}

	apifile := filepath.Join(ipath, "api")

	data, err := ioutil.ReadFile(apifile)
	if err != nil {
		return nil, err
	}

	addr := strings.Trim(string(data), "\n\t ")

	host, err := multiaddrToNormal(addr)
	if err != nil {
		return nil, err
	}

	local := sh.NewShell(host)

	_, _, err = local.Version()
	if err != nil {
		return nil, err
	}

	return local, nil
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
