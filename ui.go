package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

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
	if filter == "" {
		return true
	}

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

const (
	tBar  = "│"
	tEnd  = "└"
	tDash = "─"
	tTree = "├"
)

func (dtn *depTreeNode) printFiltered(filter string, quiet bool) {
	tabw := tabwriter.NewWriter(os.Stdout, 12, 4, 1, ' ', 0)

	var rec func(*depTreeNode, string)
	rec = func(p *depTreeNode, prefix string) {
		var toprint []*depTreeNode
		for _, n := range p.children {
			if n.matches(filter) {
				toprint = append(toprint, n)
			}
		}
		for i, n := range toprint {
			last := i == len(toprint)-1
			dep := n.this
			label := dep.Hash
			if !quiet {
				pref := tTree
				if last {
					pref = tEnd
				}

				label = fmt.Sprintf("%s%s \033[1m%s\033[0m\t%s\t%s", pref, tDash, dep.Name, dep.Hash, dep.Version)
			}

			fmt.Fprintln(tabw, prefix+label)

			nextPref := prefix + tBar + "  "
			if last {
				nextPref = prefix + "   "
			}
			rec(n, nextPref)
		}
	}

	rec(dtn, "")

	tabw.Flush()
}

func printDepsTree(pm *gx.PM, pkg *gx.Package, quiet bool) error {
	t, err := genDepsTree(pm, pkg)
	if err != nil {
		return err
	}

	t.printFiltered("", quiet)

	return nil
}
