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
	outs, err := strconv.Unquote(string(out)) // for printing out raw strings
	if err != nil {
		outs = string(out)
	}
	Log(outs)
}

type depTreeNode struct {
	this     *gx.Dependency
	children []*depTreeNode
}

func genDepsTree(pm *gx.PM, pkg *gx.Package) (*depTreeNode, error) {
	cur := new(depTreeNode)

	err := pkg.ForEachDep(func(dep *gx.Dependency, dpkg *gx.Package) error {
		sub, err := genDepsTree(pm, dpkg)
		if err != nil {
			return err
		}

		sub.this = dep
		cur.children = append(cur.children, sub)

		return nil
	})

	return cur, err
}

func (dtn *depTreeNode) matches(filter string) bool {
	if dtn.this.Hash == filter || dtn.this.Name == filter {
		return true
	}

	for _, c := range dtn.children {
		if c.matches(filter) {
			return true
		}
	}
	return false
}

func (dtn *depTreeNode) printFiltered(filter string, quiet bool) {
	var rec func(*depTreeNode, int)
	rec = func(p *depTreeNode, indent int) {
		for _, n := range p.children {
			if !n.matches(filter) {
				continue
			}
			dep := n.this
			label := dep.Hash
			if !quiet {
				label = fmt.Sprintf("%s %s %s", dep.Name, dep.Hash, dep.Version)
			}
			Log("%s%s", strings.Repeat("  ", indent), label)

			rec(n, indent+1)
		}
	}

	rec(dtn, 0)
}

func printDepsTree(pm *gx.PM, pkg *gx.Package, quiet bool, indent int) error {
	return pkg.ForEachDep(func(dep *gx.Dependency, dpkg *gx.Package) error {
		label := dep.Hash
		if !quiet {
			label = fmt.Sprintf("%s %s %s", dep.Name, dep.Hash, dep.Version)
		}
		Log("%s%s", strings.Repeat("  ", indent), label)

		err := printDepsTree(pm, dpkg, quiet, indent+1)
		if err != nil {
			return err
		}

		return nil
	})
}
