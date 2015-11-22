package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	gx "github.com/whyrusleeping/gx/gxutil"
	. "github.com/whyrusleeping/stump"
)

func promptUser(query string) string {
	fmt.Printf("%s ", query)
	scan := bufio.NewScanner(os.Stdin)
	scan.Scan()
	return scan.Text()
}

func yesNoPrompt(prompt string, def bool) bool {
	opts := "[y/N]"
	if def {
		opts = "[Y/n]"
	}

	fmt.Printf("%s %s ", prompt, opts)
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		val := strings.ToLower(scan.Text())
		switch val {
		case "":
			return def
		case "y":
			return true
		case "n":
			return false
		default:
			fmt.Println("please type 'y' or 'n'")
		}
	}

	panic("unexpected termination of stdin")
}

func jsonPrint(i interface{}) {
	out, _ := json.MarshalIndent(i, "", "  ")
	outs, err := strconv.Unquote(string(out))
	if err != nil {
		outs = string(out)
	}
	Log(outs)
}

func printDepsTree(pm *gx.PM, pkg *gx.Package, quiet bool, indent int) error {
	for _, d := range pkg.Dependencies {
		label := d.Hash
		if !quiet {
			label = fmt.Sprintf("%s %s %s", d.Name, d.Hash, d.Version)
		}
		Log("%s%s", strings.Repeat("  ", indent), label)
		npkg, err := pm.GetPackage(d.Hash)
		if err != nil {
			return err
		}

		err = printDepsTree(pm, npkg, quiet, indent+1)
		if err != nil {
			return err
		}
	}
	return nil
}
