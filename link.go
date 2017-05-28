package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// in go-ipfs: go-libp2p QmRai5yZNL67pWCoznW7sBdFnqZrFULuJ5w8KhmRyhdgN4 4.3.11

var LinkCommand = cli.Command{
	Name:        "link",
	Usage:       "TODO: usage",
	Description: `TODO: write some prose here.`,
	Action: func(c *cli.Context) error {
		// rm -rf $GOPATH/src/gx/ipfs/$hash
		// go get $dvcsimport
		// ln -s $GOPATH/src/$dvcsimport $GOPATH/src/gx/ipfs/$hash
		// cd $GOPATH/src/gx/ipfs/$hash && gx install && gx-go rewrite

		subjs := c.Args()
		if len(subjs) == 0 {
			return listLinkedPackages()
		}

		var pkg gx.Package
		err := gx.LoadPackageFile(&pkg, gx.PkgFileName)
		if err != nil {
			return err
		}

		for _, s := range subjs {

			err := linkPackage(hash, importpath)
			if err != nil {
				return err
			}
			fmt.Printf("linked %s => %s", hash, importpath)
		}

		return nil
	},
}

var UnlinkCommand = cli.Command{
	Name:        "unlink",
	Usage:       "TODO: usage",
	Description: `TODO: write some prose here.`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "all,a",
			Usage: "TODO: usage",
		},
	},
	Action: func(c *cli.Context) error {
		// unlink $GOPATH/src/gx/ipfs/$hash
		// gx install $hash
		return nil
	},
}

func listLinkedPackages() error {
	return nil
}

func linkPackage(hash string, importpath string) error {
	return nil
}

func unlinkPackage(hash string) error {
	return nil
}

func GxDvcsImport(pkg *gx.Package) string {
	pkggx := make(map[string]interface{})
	_ = json.Unmarshal(pkg.Gx, &pkggx)
	return pkggx["dvcsimport"].(string)
}

func PkgDir(pkg *gx.Package) (string, error) {
	dir, err := gx.InstallPath(pkg.Language, "", true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, GxDvcsImport(pkg)), nil
}
